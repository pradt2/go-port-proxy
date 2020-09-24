export CGO_ENABLED=0

.PHONY: all server agent

all: server agent

all-docker: server-docker agent-docker

server-docker: server
	docker build -t pp_server -f Dockerfile-server .

agent-docker: agent
	docker build -t pp_agent -f Dockerfile-agent .

server:
	go build -a -ldflags '-extldflags "-static"' -o ./bin/server.exe server.go

agent:
	GOARCH=arm go build -a -ldflags '-extldflags "-static"' -o ./bin/agent.exe agent.go
