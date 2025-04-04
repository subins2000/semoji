package main

/**
 * gittu-engine - An IBus Engine in Go
 * goibus - golang implementation of libibus
 * Copyright Sarim Khan, 2016
 * Copyright Nguyen Tran Hau, 2021
 * https://github.com/sarim/goibus
 * Licensed under Mozilla Public License 1.1 ("MPL")
 *
 * Derivative Changes: Modified names, added preferences
 * Copyright Subin Siby, 2021
 */

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/subins2000/semoji/ibus"

	"github.com/godbus/dbus/v5"
)

var installPrefix = "/usr/local"

var engineName = "Semoji"
var engineCode = "semoji"

// Bus name related to the engine used which is goSemoji
var busName = "org.freedesktop.IBus.semoji"

var debug = flag.Bool("debug", false, "Enable debugging")
var embeded = flag.Bool("ibus", false, "Run the embeded ibus component")
var standalone = flag.Bool("standalone", false, "Run standalone by creating new component")

func makeComponent() *ibus.Component {
	component := ibus.NewComponent(
		busName,
		engineName+" Input Engine",
		"1.6.0",
		"AGPL-3.0",
		"Subin Siby",
		"https://subinsb.com",
		installPrefix+"/bin/semoji-ibus-engine -ibus",
		"ibus-semoji-")

	engineDesc := ibus.SmallEngineDesc(
		engineCode,
		engineName,
		engineName+" Input Method",
		"en",
		"AGPL-3.0",
		"Subin Siby",
		installPrefix+"/share/semoji/icon.png",
		"en",
		installPrefix+"/bin/semoji-ibus-engine -prefs",
		"1.6.0")

	component.AddEngine(engineDesc)

	return component
}

func main() {
	if *debug {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	var Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			format := "  -%s: %s\n"
			fmt.Fprintf(os.Stderr, format, f.Name, f.Usage)
		})
	}

	flag.Parse()

	if *embeded {
		bus := ibus.NewBus()
		fmt.Println("Got Bus, Running Embeded")

		conn := bus.GetDbusConn()
		ibus.NewFactory(conn, SemojiEngineCreator)
		bus.RequestName(busName, 0)
		select {}
	} else if *standalone {
		bus := ibus.NewBus()
		fmt.Println("Got Bus, Running Standalone")

		conn := bus.GetDbusConn()
		ibus.NewFactory(conn, SemojiEngineCreator)
		bus.RegisterComponent(makeComponent())

		fmt.Println("Setting Global Engine to me")
		bus.CallMethod("SetGlobalEngine", 0, "Semoji")

		c := make(chan *dbus.Signal, 10)
		conn.Signal(c)

		select {
		case <-c:
		}

	} else {
		Usage()
		os.Exit(1)
	}
}
