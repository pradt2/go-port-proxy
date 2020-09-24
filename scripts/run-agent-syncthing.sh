#!/usr/bin/env bash
docker container rm -f pp_agent_syncthing
docker run \
        --name pp_agent_syncthing \
        --net host \
        -e APP_CONTROL_CONN_ADDR="api.thinkthing.xyz:9003" \
        -e APP_TRANSFER_CONN_ADDR="api.thinkthing.xyz:8503" \
        -e APP_LOCAL_CONN_ADDR=":8003" \
        --restart always \
        -d \
        pp_agent
