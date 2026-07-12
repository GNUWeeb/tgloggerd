#!/bin/bash
set -xe;
export TG_API_ID="123456";                             # from https://my.telegram.org
export TG_API_HASH="0123456789abcdef0123456789abcdef"; # from https://my.telegram.org
export TG_DATA_DIR="./data/users/example";             # TDLib data directory
export TG_LOG_FILE="";                                 # empty logs to stdout
export TG_LOG_LEVEL="debug";                           # error | warning | info | debug
chrt --idle 0 nice -n 19 ionice -c 3 bash -c "cmake -B build && cmake --build build -j$(nproc)";
exec build/tgloggerd;
