#!/bin/bash

osdist=$(uname -s)

if [ "$osdist" == "Linux" ]; then
  ./dcrcli-linux 
elif [ "$osdist" == "Darwin" ]; then
  ./dcrcli-osx 
else
  echo "Need Linux or macOS"
fi
