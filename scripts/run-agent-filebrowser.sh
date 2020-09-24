#!/usr/bin/env bash
docker container rm -f pp_agent_filebrowser
docker run \
        --name pp_agent_filebrowser \
        --net host \
        -e APP_CONTROL_CONN_ADDR="api.thinkthing.xyz:9002" \
        -e APP_TRANSFER_CONN_ADDR="api.thinkthing.xyz:8502" \
        -e APP_LOCAL_CONN_ADDR=":8002" \
        --restart always \
        -d \
        pp_agent
