package main

import (
	"errors"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

//***This is my first time writing go so please excuse me if you see any go antipatterns!***
/*
I became aware of https://godoc.org/github.com/sparrc/go-ping during my research but I
thought it was a bit too high level and that you probably want to see a lower level solution
with more raw icmp requests.
*/
const (
	protocolICMP   = 1    // IPV4 protocol number
	ProtocolICMPV6 = 58   // IPV6 protocol number
	defaultTimeout = "3s" // Default read timeout
)

type stats struct {
	totalTime  int64
	totalSent  int64
	lost       int64
	avgLatency float64
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please specify a hostname or ip to ping through cmdline args")
	}
	hostnameOrIP := os.Args[1]
	fmt.Println("hostname or ip entered: ", hostnameOrIP)
	resolvedIP, e := getIP(hostnameOrIP)
	if e != nil {
		log.Fatal(e)
	}
	log.Printf("Resolved to: %s", resolvedIP.String())
	var protocol = protocolICMP
	conn := getListener(protocol)
	defer conn.Close()
	messageBinary := getMessage(protocol)
	s := stats{lost: 0, totalTime: 0}
	for {
		t := writeAndListen(conn, messageBinary, resolvedIP, protocol, &s)
		if t != -1 {
			log.Printf("RTT: %d ms", t)
		}
	}
}

func getIP(ipOrHostname string) (net.IP, error) {
	//https://stackoverflow.com/questions/5284147/validating-ipv4-addresses-with-regexp
	isIpv4, e := regexp.MatchString(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`, ipOrHostname)
	if e != nil {
		fmt.Println("ipv4 regex err", e)
		return nil, e
	}
	//https://stackoverflow.com/questions/53497/regular-expression-that-matches-valid-ipv6-addresses
	isIpv6, e := regexp.MatchString(`(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`, ipOrHostname)
	if e != nil {
		fmt.Println("ipv6 regex err", e)
		return nil, e
	}
	if isIpv4 && isIpv6 {
		fmt.Println("Problem with regex, IP can not be both v4 and v6")
		return nil, errors.New("Problem with regex, IP can not be both v4 and v6")
	}
	//Lets get a parsed IP to use for our networking
	var parsedIP net.IP
	if !isIpv4 && !isIpv6 {
		//Resolve hostname
		parsedIPS, e := net.LookupIP(ipOrHostname)
		if e != nil {
			fmt.Println("Error while looking up hostname", e)
			return nil, e
		}
		parsedIP = parsedIPS[0]
	} else {
		parsedIP = net.ParseIP(ipOrHostname)
	}
	return parsedIP, nil
}

func getListener(protocol int) *icmp.PacketConn {
	switch protocol {
	case protocolICMP:
		conn, e := icmp.ListenPacket("ip4:1", "0.0.0.0")
		if e != nil {
			log.Fatal("Error in listenPacket", e)
		}
		return conn
	case ProtocolICMPV6:
		log.Fatal("Not implemented")
	}
	return nil
}

func getMessage(protocol int) []byte {
	switch protocol {
	case protocolICMP:
		//Even though it was ipv6, https://godoc.org/golang.org/x/net/icmp#PacketConn example was very helpful
		message := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  1,
				Data: []byte("Hi-Pinging"),
			},
		}
		messageBinary, e := message.Marshal(nil)
		if e != nil {
			log.Fatal("Error in marshalling message: ", e)
		}
		return messageBinary
	case ProtocolICMPV6:
		log.Fatal("Not implemented")
	}
	return nil
}

func writeAndListen(conn *icmp.PacketConn, messageBinary []byte, resolvedIP net.IP, protocol int, s *stats) int64 {
	start := time.Now()
	switch protocol {
	case protocolICMP:
		_, e := conn.WriteTo(messageBinary, &net.IPAddr{IP: resolvedIP})
		if e != nil {
			log.Fatal("Error in writing message: ", e)
		}
		s.totalSent++
		incomingBuf := make([]byte, 1500)
		timeout, e := time.ParseDuration(defaultTimeout)
		conn.SetReadDeadline(start.Add(timeout))
		n, peer, e := conn.ReadFrom(incomingBuf)
		if e != nil && strings.Contains(e.Error(), "i/o timeout") {
			log.Printf("Read timeout, packet considered lost")
			s.lost++
			logStats(*s)
			return -1
		} else if e != nil {
			log.Fatal("Fatal error while reading incoming message")
		}
		t := time.Now()
		elapsed := t.Sub(start).Milliseconds()
		incMessage, e := icmp.ParseMessage(protocolICMP, incomingBuf[:n])
		if e != nil {
			log.Fatal(e)
		}
		switch incMessage.Type {
		case ipv4.ICMPTypeEchoReply:
			log.Printf("Got echo reply from %v", peer)
		default:
			log.Printf("got non-icmp echo reply: %+v", incMessage)
		}
		s.totalTime += elapsed
		s.avgLatency = (s.avgLatency*float64(s.totalSent-s.lost-1) + float64(elapsed)) / float64(s.totalSent-s.lost)
		logStats(*s)
		return elapsed
	}
	return 0
}

func logStats(s stats) {
	log.Printf("Total packets sent: %d", s.totalSent)
	log.Printf("Total packets recieved: %d", s.totalSent-s.lost)
	log.Printf("Packet loss so far: %d%%", (s.lost/s.totalSent)*100)
	log.Printf("Average latency: %f ms", s.avgLatency)
}
