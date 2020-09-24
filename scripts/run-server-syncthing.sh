#!/usr/bin/env bash
docker container rm -f pp_server_syncthing
docker run \
        --name pp_server_syncthing \
        --net host \
        -e APP_CONTROL_CONN_ADDR=":9003" \
        -e APP_TRANSFER_CONN_ADDR=":8503" \
        -e APP_INCOMING_CONN_ADDR=":8003" \
        --restart always \
        -d \
        pp_server
