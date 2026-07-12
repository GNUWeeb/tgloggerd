#!/bin/bash
set -xe;
export TG_API_ID="123456";                             # from https://my.telegram.org
export TG_API_HASH="0123456789abcdef0123456789abcdef"; # from https://my.telegram.org
export TG_DATA_DIR="./data/users/example";             # TDLib data directory
export TG_LOG_FILE="";                                 # empty logs to stdout
export TG_LOG_LEVEL="debug";                           # error | warning | info | debug
export TG_DB_HOST="10.0.88.3";                         # MySQL host
export TG_DB_PORT="3306";                              # MySQL port
export TG_DB_USER="tgloggerd";                         # MySQL user
export TG_DB_PASSWORD="tgloggerd";                     # MySQL password
export TG_DB_NAME="tgloggerd";                         # MySQL database name
chrt --idle 0 nice -n 19 ionice -c 3 bash -c "cmake -B build && cmake --build build -j$(nproc)";
exec build/tgloggerd;
