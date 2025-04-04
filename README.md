# Semoji IME

For development:

* Make symlinks so that iBus knows our new engine:
```
sudo ln -s $(realpath semoji.xml) /usr/share/ibus/component/semoji.xml
sudo ln -s $(realpath semoji) /usr/local/bin/semoji-ibus-engine
```

* Open iBus settings, add our new input method, search for "semoji"

* Run
```
./build.sh && ./semoji -ibus
```

* Switch the current iBus input method to semoji