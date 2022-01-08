#! /bin/sh

/headless-shell/headless-shell --no-sandbox --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 &
./server
