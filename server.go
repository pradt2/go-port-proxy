package main

import (
	"project-proxy/messaging"
	"time"
	"project-proxy/server"
	"flag"
	"github.com/jamiealquiza/envy"
	"project-proxy/logs"
	"project-proxy/connectivity"
	"project-proxy/certs"
)

func main() {
	controlConnNetworkType := flag.String("control-conn-net-type", "tcp", "The network type of the control connection")
	controlConnAddress := flag.String("control-conn-addr", ":9001", "The ip_addr:port combination of the control connection")
	controlConnRestartInterval := flag.Int("control-conn-reset-interval", 1000, "Waiting time in ms before attempting to restart the control connection")
	controlConnPingInterval := flag.Int("control-conn-ping-interval", 30000, "Waiting time in ms between pings to keep the control connection alive. Setting this to zero disables pinging")
	controlConnPingTimeout := flag.Int("control-conn-ping-timeout", 45000, "Max waiting time in ms for a ping message before the control connection gets closed and re-established. Setting this to zero disables timeout")
	incomingConnNetworkType := flag.String("incoming-conn-net-type", "tcp", "The network type of the incoming client connections")
	incomingConnAddress := flag.String("incoming-conn-addr", ":80", "The ip_addr:port combination of the incoming client connections")
	transferConnNetworkType := flag.String("transfer-conn-net-type", "tcp", "The network type of the transfer connections")
	transferConnAddress := flag.String("transfer-conn-addr", ":8888", "The ip_addr:port combination of the transfer connections")
	bufferSize := flag.Uint64("buffer-size", 512, "Size of the buffer (in KB) used to read from and write to remote connections")
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

	incomingCf := connectivity.NewTCPConnectionFactory(*incomingConnNetworkType, *incomingConnAddress)

	log.Infof("Trying to listen for a type %s control connection at %s", *controlConnNetworkType, *controlConnAddress)
	ln, err := controlCf.Listen()
	if err != nil {
		log.Fatalf("Could not listen for control connection. Cause: %s", err)
	}
	for {
		log.Infof("Successfully listening for an agent to establish a control connection")
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("Could not accept a control connection. Cause: %s", err)
			continue
		} else {
			log.Infof("Successfully established a control connection with agent addr: %s Starting the server", conn.RemoteAddr())
			mess := messaging.NewMessenger(conn)
			mess.SetTimeout(time.Duration(*controlConnPingTimeout) * time.Millisecond)
			s := server.NewServer(incomingCf, transferCf,
				time.Duration(*controlConnPingInterval)*time.Millisecond,
				*bufferSize*1024, messaging.NewMessengerOverlay(mess))
			s.Start()
			s.Wait()
			log.Warningf("The server has finished. This usually means connectivity or agent problems. Allowing another control connection")
		}
		sleepingTime := time.Millisecond * time.Duration(*controlConnRestartInterval)
		log.Warningf("Waiting for %d ms to allow the next control connection", sleepingTime/time.Millisecond)
		time.Sleep(sleepingTime)
	}
}
