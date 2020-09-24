#!/usr/bin/env bash
docker container rm -f pp_agent_ssh
docker run \
        --name pp_agent_ssh \
        --net host \
        -e APP_CONTROL_CONN_ADDR="api.thinkthing.xyz:9001" \
        -e APP_TRANSFER_CONN_ADDR="api.thinkthing.xyz:8501" \
        -e APP_LOCAL_CONN_ADDR=":22" \
        --restart always \
        -d \
        pp_agent
