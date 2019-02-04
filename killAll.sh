#!/bin/bash

ps -ef | grep 'go run main.go ./www' | sort | head -n 1 | awk '{split($0,a," "); print a[2]}' | xargs kill
ps -ef | grep 'main ./www' | sort | head -n 1 | awk '{split($0,a," "); print a[2]}' | xargs kill
ps -ef | grep 'ffmpeg/ffmpeg' | grep 'dash' | head -n 1 | awk '{split($0,a," "); print a[2]}' | xargs kill

echo All processes have been killed ☠
