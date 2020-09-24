package main

import (
	"project-proxy/certs"
	"time"
	"project-proxy/messaging"
	"flag"
	"github.com/jamiealquiza/envy"
	"project-proxy/agent"
	"project-proxy/logs"
	"project-proxy/connectivity"
)

func main() {
	controlConnNetworkType := flag.String("control-conn-net-type", "tcp", "The network type of the control connection")
	controlConnAddress := flag.String("control-conn-addr", ":9001", "The ip_addr:port combination of the control connection")
	controlConnRestartInterval := flag.Int("control-conn-reset-interval", 1000, "Waiting time in ms before attempting to restart the control connection")
	controlConnPingInterval := flag.Int("control-conn-ping-interval", 30000, "Waiting time in ms between pings to keep the control connection alive. Setting this to zero disables pinging")
	controlConnPingTimeout := flag.Int("control-conn-ping-timeout", 45000, "Max waiting time in ms for a ping message before the control connection gets closed and re-established. Setting this to zero disables timeout")
	transferConnNetworkType := flag.String("transfer-conn-net-type", "tcp", "The network type of the transfer connections")
	transferConnAddress := flag.String("transfer-conn-addr", ":8888", "The ip_addr:port combination of the transfer connections")
	localConnNetworkType := flag.String("local-conn-net-type", "tcp", "The network type of the incoming client connections")
	localConnAddress := flag.String("local-conn-addr", ":80", "The ip_addr:port combination of the incoming client connections")
	bufferSize := flag.Uint64("buffer-size", 512, "Size of the buffer (in KB) used to read from and write to local connections")
	logLevel := flag.Int("log-level", 4, "Log levels: 0 CRITICAL, 1 ERROR, 2 WARNING, 3 NOTICE, 4 INFO, 5 DEBUG")
	usePlainTcpTransferConns := flag.Bool("tcp-transfer-conns", false, "If true, uses plain TCP (instead of TLS) for transfer connections. This is usually a bad idea")

	envy.Parse("APP")
	flag.Parse()

	logs.Init(*logLevel)
	log := logs.GetLoggerForModule("main")

	var controlCf connectivity.ConnFactory
	var transferCf connectivity.ConnFactory
	if *usePlainTcpTransferConns {
		controlCf = connectivity.NewTCPConnectionFactory(*controlConnNetworkType, *controlConnAddress)
		transferCf = connectivity.NewTCPConnectionFactory(*transferConnNetworkType, *transferConnAddress)
		log.Warning("Plain TCP is used for transfer connections. Please check if this is as intended")
	} else {
		controlCf = connectivity.NewTLSConnectionFactory(certs.RootCertificate, certs.ServerPrivateKey,
			certs.ServerCertificate, true, *controlConnNetworkType, *controlConnAddress)
		transferCf = connectivity.NewTLSConnectionFactory(certs.RootCertificate, certs.ServerPrivateKey,
			certs.ServerCertificate, true, *transferConnNetworkType, *transferConnAddress)
	}

	localCf := connectivity.NewTCPConnectionFactory(*localConnNetworkType, *localConnAddress)

	for {
		log.Infof("Trying to establish a type: %s control connection with a server at: %s", *controlConnNetworkType, *controlConnAddress)
		conn, err := controlCf.Connect()
		if err != nil {
			log.Errorf("Could not connect to the server. Cause: %s", err)
		} else {
			log.Infof("Successfully connected to the server. Starting the agent")
			mess := messaging.NewMessenger(conn)
			mess.SetTimeout(time.Duration(*controlConnPingTimeout) * time.Millisecond)
			a := agent.NewAgent(localCf, transferCf,
				time.Duration(*controlConnPingInterval)*time.Millisecond,
				*bufferSize*uint64(1024), messaging.NewMessengerOverlay(mess))
			a.Start()
			a.Wait()
			log.Warningf("The agent has finished. This usually means connectivity or server problems. Reconnecting")
		}
		sleepingTime := time.Duration(*controlConnRestartInterval) * time.Millisecond
		log.Warningf("Waiting for %d ms to reconnect to the server", sleepingTime/time.Millisecond)
		time.Sleep(sleepingTime)
	}

}
