#!/usr/bin/env bash
docker container rm -f pp_server_filebrowser
docker run \
        --name pp_server_filebrowser \
        --net host \
        -e APP_CONTROL_CONN_ADDR=":9002" \
        -e APP_TRANSFER_CONN_ADDR=":8502" \
        -e APP_INCOMING_CONN_ADDR=":8002" \
        --restart always \
        -d \
        pp_server
