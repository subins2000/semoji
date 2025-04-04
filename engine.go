package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/subins2000/semoji/ibus"

	"github.com/godbus/dbus/v5"
)

type SemojiEngine struct {
	ibus.Engine
	propList          *ibus.PropList
	preedit           []rune
	cursorPos         uint32
	table             *ibus.LookupTable
	transliterateCTX  context.Context
	updateTableCancel context.CancelFunc
}

type Emoji struct {
	Symbol      string
	Name        string
	Description string
	Keywords    []string
}

var emojiTable = []Emoji{
	{
		Symbol:      "😀",
		Name:        "grinning",
		Description: "Grinning face",
		Keywords:    []string{"smile", "happy", "joy", "grin"},
	},
	{
		Symbol:      "🔥",
		Name:        "fire",
		Description: "Fire emoji",
		Keywords:    []string{"hot", "lit", "flame"},
	},
	{
		Symbol:      "🎉",
		Name:        "party popper",
		Description: "Celebration or party",
		Keywords:    []string{"celebrate", "party", "yay"},
	},
}

func (e *SemojiEngine) SemojiUpdatePreedit() {
	e.UpdatePreeditText(ibus.NewText(string(e.preedit)), e.cursorPos, true)
}

func (e *SemojiEngine) SemojiClearState() {
	e.preedit = []rune{}
	e.cursorPos = 0
	e.SemojiUpdatePreedit()

	e.table.Clear()
	e.HideLookupTable()
}

func (e *SemojiEngine) SemojiCommitText(text *ibus.Text, shouldLearn bool) bool {
	e.CommitText(text)
	e.SemojiClearState()
	return true
}

func getSuggestions(ctx context.Context, channel chan<- []string, query string) {
	result := []string{}

	for _, emoji := range emojiTable {
		for _, keyword := range emoji.Keywords {
			if strings.Contains(strings.ToLower(keyword), query) {
				result = append(result, emoji.Symbol)
				break
			}
		}
	}

	channel <- result
	close(channel)
}

func (e *SemojiEngine) InternalUpdateTable(ctx context.Context) {
	resultChannel := make(chan []string)

	go getSuggestions(ctx, resultChannel, string(e.preedit))

	select {
	case <-ctx.Done():
		return
	case result := <-resultChannel:
		e.table.Clear()

		for _, suggestion := range result {
			e.table.AppendCandidate(suggestion)
		}

		label := uint32(1)
		for label <= e.table.PageSize {
			e.table.AppendLabel(fmt.Sprint(label) + ":")
			label++
		}

		e.UpdateLookupTable(e.table, true)
	}
}

func (e *SemojiEngine) SemojiUpdateLookupTable() {
	if e.updateTableCancel != nil {
		e.updateTableCancel()
		e.updateTableCancel = nil
	}

	if len(e.preedit) == 0 {
		e.HideLookupTable()
		return
	}

	ctx, cancel := context.WithCancel(e.transliterateCTX)
	e.updateTableCancel = cancel

	e.InternalUpdateTable(ctx)
}

func (e *SemojiEngine) GetCandidateAt(index uint32) *ibus.Text {
	if int(index) > len(e.table.Candidates)-1 {
		return nil
	}
	text := e.table.Candidates[index].Value().(ibus.Text)
	return &text
}

func (e *SemojiEngine) GetCandidate() *ibus.Text {
	return e.GetCandidateAt(e.table.CursorPos)
}

func (e *SemojiEngine) SemojiCommitCandidateAt(index uint32) (bool, *dbus.Error) {
	page := uint32(e.table.CursorPos / e.table.PageSize)

	index = page*e.table.PageSize + index

	if *debug {
		fmt.Println("Pagination picker:", len(e.table.Candidates), e.table.CursorPos, page, index)
	}

	text := e.GetCandidateAt(uint32(index))
	if text != nil {
		return e.SemojiCommitText(text, true), nil
	}
	return false, nil
}

func isWordBreak(ukeyval uint32) bool {
	keyval := int(ukeyval)
	// 46 is .
	// 44 is ,
	// 63 is ?
	// 33 is !
	// 40 is (
	// 41 is )
	if keyval == 46 || keyval == 44 || keyval == 63 || keyval == 33 || keyval == 40 || keyval == 41 {
		return true
	}
	// 59 is ;
	// 39 is '
	// 34 is "
	if keyval == 59 || keyval == 39 || keyval == 34 {
		return true
	}
	return false
}

func (e *SemojiEngine) ProcessKeyEvent(keyval uint32, keycode uint32, modifiers uint32) (bool, *dbus.Error) {
	if *debug {
		fmt.Println("Process Key Event > ", keyval, keycode, modifiers)
	}

	// Ignore key release events
	is_press := modifiers&ibus.IBUS_RELEASE_MASK == 0
	if !is_press {
		return false, nil
	}

	altModifiers := modifiers & ibus.IBUS_MOD1_MASK
	if altModifiers != 0 {
		if len(e.preedit) == 0 {
			return false, nil
		}
		if keyval == ibus.IBUS_Down {
			if *debug {
				fmt.Println("ALT + DOWN = Suggestions page down")
			}
			e.table.NextPage()
		} else if keyval == ibus.IBUS_Up {
			if *debug {
				fmt.Println("ALT + UP = Suggestions page up")
			}
			e.table.PreviousPage()
		}
		e.UpdateLookupTable(e.table, true)
		return true, nil
	}

	switch keyval {
	case ibus.IBUS_Space:
		text := e.GetCandidate()
		if text == nil {
			e.SemojiCommitText(ibus.NewText(string(e.preedit)+" "), false)
		} else {
			e.SemojiCommitText(ibus.NewText(text.Text+" "), true)
		}
		return true, nil

	case ibus.IBUS_Return:
		text := e.GetCandidate()
		if text == nil {
			e.SemojiCommitText(ibus.NewText(string(e.preedit)), false)
			return false, nil
		} else {
			e.SemojiCommitText(text, true)
		}
		return true, nil

	case ibus.IBUS_Escape:
		if len(e.preedit) == 0 {
			return false, nil
		}
		e.SemojiCommitText(ibus.NewText(string(e.preedit)), false)
		return true, nil

	case ibus.IBUS_Left:
		if len(e.preedit) == 0 {
			return false, nil
		}
		if e.cursorPos > 0 {
			e.cursorPos--
			e.SemojiUpdatePreedit()
		}
		return true, nil

	case ibus.IBUS_Right:
		if len(e.preedit) == 0 {
			return false, nil
		}
		if int(e.cursorPos) < len(e.preedit) {
			e.cursorPos++
			e.SemojiUpdatePreedit()
		}
		return true, nil

	case ibus.IBUS_Up:
		if len(e.preedit) == 0 {
			return false, nil
		}
		e.table.CursorUp()
		e.UpdateLookupTable(e.table, true)
		return true, nil

	case ibus.IBUS_Down:
		if len(e.preedit) == 0 {
			return false, nil
		}
		e.table.CursorDown()
		e.UpdateLookupTable(e.table, true)
		return true, nil

	case ibus.IBUS_BackSpace:
		if len(e.preedit) == 0 {
			return false, nil
		}
		if e.cursorPos > 0 {
			e.cursorPos--
			e.preedit = removeAtIndex(e.preedit, e.cursorPos)
			e.SemojiUpdatePreedit()
			e.SemojiUpdateLookupTable()
			if len(e.preedit) == 0 {
				/* Current backspace has cleared the preedit. Need to reset the engine state */
				e.SemojiClearState()
			}
		}
		return true, nil

	case ibus.IBUS_Delete:
		if len(e.preedit) == 0 {
			return false, nil
		}
		if int(e.cursorPos) < len(e.preedit) {
			e.preedit = removeAtIndex(e.preedit, e.cursorPos)
			e.SemojiUpdatePreedit()
			e.SemojiUpdateLookupTable()
			if len(e.preedit) == 0 {
				/* Current delete has cleared the preedit. Need to reset the engine state */
				e.SemojiClearState()
			}
		}
		return true, nil

	case ibus.IBUS_Home, ibus.IBUS_KP_Home:
		if len(e.preedit) == 0 {
			return false, nil
		}
		e.cursorPos = 0
		e.SemojiUpdatePreedit()
		return true, nil

	case ibus.IBUS_End, ibus.IBUS_KP_End:
		if len(e.preedit) == 0 {
			return false, nil
		}
		e.cursorPos = uint32(len(e.preedit))
		e.SemojiUpdatePreedit()
		return true, nil

	case ibus.IBUS_0, ibus.IBUS_KP_0:
		if len(e.preedit) == 0 {
			return false, nil
		}
		// Commit the text itself
		e.SemojiCommitText(ibus.NewText(string(e.preedit)), false)
		return true, nil
	}

	numericKey := uint32(10)

	switch keyval {
	case ibus.IBUS_1, ibus.IBUS_KP_1:
		numericKey = 0
		break
	case ibus.IBUS_2, ibus.IBUS_KP_2:
		numericKey = 1
		break
	case ibus.IBUS_3, ibus.IBUS_KP_3:
		numericKey = 2
		break
	case ibus.IBUS_4, ibus.IBUS_KP_4:
		numericKey = 3
		break
	case ibus.IBUS_5, ibus.IBUS_KP_5:
		numericKey = 4
		break
	case ibus.IBUS_6, ibus.IBUS_KP_6:
		numericKey = 5
		break
	case ibus.IBUS_7, ibus.IBUS_KP_7:
		numericKey = 6
		break
	case ibus.IBUS_8, ibus.IBUS_KP_8:
		numericKey = 7
		break
	case ibus.IBUS_9, ibus.IBUS_KP_9:
		numericKey = 8
	}

	if numericKey != 10 {
		return e.SemojiCommitCandidateAt(numericKey)
	}

	if isWordBreak(keyval) {
		text := e.GetCandidate()
		if text != nil {
			e.SemojiCommitText(ibus.NewText(text.Text+string(keyval)), true)
			return true, nil
		}
		return false, nil
	}

	if keyval <= 128 {
		if len(e.preedit) == 0 {
			/* We are starting a new word. Now there could be a word selected in the text field
			 * and we may be typing over the selection. In this case to clear the selection
			 * we commit a empty text which will trigger the textfield to clear the selection.
			 * If there is no selection, this won't affect anything */
			e.CommitText(ibus.NewText(""))
		}

		// Appending at cursor position
		e.preedit = insertAtIndex(e.preedit, e.cursorPos, rune(keyval))
		e.cursorPos++

		e.SemojiUpdatePreedit()

		e.SemojiUpdateLookupTable()

		return true, nil
	}
	return false, nil
}

func (e *SemojiEngine) FocusIn() *dbus.Error {
	e.RegisterProperties(e.propList)
	return nil
}

func (e *SemojiEngine) FocusOut() *dbus.Error {
	e.SemojiClearState()
	return nil
}

func (e *SemojiEngine) PropertyActivate(prop_name string, prop_state uint32) *dbus.Error {
	fmt.Println("PropertyActivate", prop_name)
	return nil
}

func (c *SemojiEngine) Destroy() *dbus.Error {
	return nil
}

var eid = 0

func SemojiEngineCreator(conn *dbus.Conn, engineName string) dbus.ObjectPath {
	eid++
	fmt.Println("Creating Semoji Engine #", eid)
	objectPath := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/IBus/Engine/Semoji/%d", eid))

	propp := ibus.NewProperty(
		"setup",
		ibus.PROP_TYPE_NORMAL,
		"Preferences - Semoji",
		"gtk-preferences",
		"Configure Semoji Engine",
		true,
		true,
		ibus.PROP_STATE_UNCHECKED)

	engine := &SemojiEngine{
		ibus.BaseEngine(conn, objectPath),
		ibus.NewPropList(propp),
		[]rune{},
		0,
		ibus.NewLookupTable(),
		context.Background(),
		nil}

	// TODO add SetOrientation method
	// engine.table.emitSignal("SetOrientation", ibus.IBUS_ORIENTATION_VERTICAL)

	ibus.PublishEngine(conn, objectPath, engine)
	return objectPath
}

func removeAtIndex(s []rune, index uint32) []rune {
	return append(s[0:index], s[index+1:]...)
}

// Thanks wasmup https://stackoverflow.com/a/61822301/1372424
// 0 <= index <= len(a)
func insertAtIndex(a []rune, index uint32, value rune) []rune {
	if uint32(len(a)) == index { // nil or empty slice or after last element
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}
