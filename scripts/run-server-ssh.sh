#!/usr/bin/env bash
docker container rm -f pp_server_ssh
docker run \
	--name pp_server_ssh \
	--net host \
	-e APP_CONTROL_CONN_ADDR=":9001" \
	-e APP_TRANSFER_CONN_ADDR=":8501" \
	-e APP_INCOMING_CONN_ADDR=":8001" \
	--restart always \
	-d \
	pp_server
