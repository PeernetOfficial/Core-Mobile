# Peernet Mobile Core
This library was created for peernet mobile changes for compatability with IOS and Android.

## Instructions to build for Android. 
1. enter into the ```mobile/``` folder. 
(Ex: on linux)
```
cd mobile/
```
2. Install go mobile 
```
go install golang.org/x/mobile/cmd/gomobile@latest
```
3. Initialize go mobile 
```
gomobile init
```
4. Add path for Android NDK 
```
export ANDROID_HOME=$HOME/Android/Sdk
```
5. Generate .aar and .jar file 
```
gomobile bind -target android .

Output:
mobile.aar  mobile-sources.jar
```

The core library which is needed for any Peernet application. It provides connectivity to the network and all basic functions. For details about Peernet see https://peernet.org/.

## Contributing

Please note that by contributing code, documentation, ideas, snippets, or any other intellectual property you agree that you have all the necessary rights and you agree that we, the Peernet organization, may use it for any purpose.

&copy; 2021 Peernet s.r.o.
