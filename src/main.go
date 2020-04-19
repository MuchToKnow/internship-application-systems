package main

import (
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
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
	protocolICMP   = 1      // IPV4 protocol number
	protocolICMPV6 = 58     // IPV6 protocol number
	defaultTimeout = "3s"   // Default read timeout
	defaultSleep   = "1.5s" // Default sleep between pings
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
	var count int = -1
	if len(os.Args) > 3 && os.Args[2] == "-c" {
		var e error
		count, e = strconv.Atoi(os.Args[3])
		if e != nil {
			log.Fatal("Error while parsing arg ", os.Args[3])
		}
	}
	//First arg is index 1 as the binary path is index 0
	hostnameOrIP := os.Args[1]
	fmt.Println("hostname or ip entered: ", hostnameOrIP)
	//Call our resolver to deal with any hostname/ip input
	resolvedIP, protocol, e := getIP(hostnameOrIP)
	if e != nil {
		log.Fatal(e)
	}
	log.Printf("Resolved to: %s", resolvedIP.String())
	//Get a listener on a network socket
	conn := getListener(protocol)
	defer conn.Close()
	//Get a valid message binary for our chosen protocol
	messageBinary := getMessage(protocol)
	s := stats{lost: 0, totalTime: 0}
	//Main loop that calls the ping function (writeandlisten)
	for {
		t := writeAndListen(conn, messageBinary, resolvedIP, protocol, &s)
		if t != -1 {
			log.Printf("RTT: %d ms", t)
		}
		sleepDur, _ := time.ParseDuration(defaultSleep)
		count--
		if count == 0 {
			break
		}
		time.Sleep(sleepDur)
	}
}

func getIP(ipOrHostname string) (net.IP, int, error) {
	//https://stackoverflow.com/questions/5284147/validating-ipv4-addresses-with-regexp
	isIpv4, e := regexp.MatchString(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`, ipOrHostname)
	if e != nil {
		log.Fatal("ipv4 regex err", e)
	}
	//https://stackoverflow.com/questions/53497/regular-expression-that-matches-valid-ipv6-addresses
	isIpv6, e := regexp.MatchString(`(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`, ipOrHostname)
	if e != nil {
		log.Fatal("ipv6 regex err", e)
	}
	if isIpv4 && isIpv6 {
		log.Fatal("Problem with regex, IP can not be both v4 and v6")
	}
	//Lets get a parsed IP to use for our networking
	var parsedIP net.IP
	if !isIpv4 && !isIpv6 {
		//Resolve hostname
		parsedIPS, e := net.LookupIP(ipOrHostname)
		if e != nil {
			log.Fatal("Error while looking up hostname", e)
		}
		parsedIP = parsedIPS[0]
		isIpv4 = true
	} else {
		parsedIP = net.ParseIP(ipOrHostname)
	}
	if isIpv4 {
		return parsedIP, protocolICMP, nil
	}
	return parsedIP, protocolICMPV6, nil
}

func getListener(protocol int) *icmp.PacketConn {
	switch protocol {
	case protocolICMP:
		conn, e := icmp.ListenPacket("ip4:1", "0.0.0.0")
		if e != nil {
			log.Fatal("Error in listenPacket", e)
		}
		return conn
	case protocolICMPV6:
		conn, e := icmp.ListenPacket("ip6:58", "::")
		if e != nil {
			log.Fatal("Error in listenPacket", e)
		}
		return conn
	}
	return nil
}

//Returns a valid ICMP echo request byte array for the given protocol
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
	case protocolICMPV6:
		message := icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
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
	}
	return nil
}

/*
The main ping function.  Writes the message binary to the ip specified and waits for a response.
Upon response, it checks response type, updates the stats struct s, prints stats, and returns.
Upon timeout, it updates the stats object s, prints stats, and returns.
*/
func writeAndListen(conn *icmp.PacketConn, messageBinary []byte, resolvedIP net.IP, protocol int, s *stats) int64 {
	start := time.Now()
	bytes, e := conn.WriteTo(messageBinary, &net.IPAddr{IP: resolvedIP})
	if e != nil {
		log.Fatal("Error in writing message: ", e)
	}
	log.Printf("Pinged %s with %d bytes", resolvedIP.String(), bytes)
	s.totalSent++

	incomingBuf := make([]byte, 1500)
	//Set a timeout for our listen so we don't listen forever
	timeout, e := time.ParseDuration(defaultTimeout)
	conn.SetReadDeadline(start.Add(timeout))
	n, peer, e := conn.ReadFrom(incomingBuf)

	//Check if we had a timeout
	if e != nil && strings.Contains(e.Error(), "i/o timeout") {
		log.Printf("Read timeout, packet considered lost")
		s.lost++
		logStats(*s)
		return -1
	} else if e != nil {
		log.Fatal("Fatal error while reading incoming message")
	}

	//Get the current time in order to calculate how long RTT was
	t := time.Now()
	elapsed := t.Sub(start).Milliseconds()
	incMessage, e := icmp.ParseMessage(protocolICMP, incomingBuf[:n])
	if e != nil {
		log.Fatal(e)
	}
	switch incMessage.Type {
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		log.Printf("Got echo reply from %v", peer)
	default:
		log.Printf("Got non-icmp echo reply: %+v", incMessage)
	}

	//Update and print statistics
	s.totalTime += elapsed
	s.avgLatency = (s.avgLatency*float64(s.totalSent-s.lost-1) + float64(elapsed)) / float64(s.totalSent-s.lost)
	logStats(*s)
	return elapsed
}

//Logs the stats contained in the stats struct s
func logStats(s stats) {
	log.Printf("Total packets sent: %d", s.totalSent)
	log.Printf("Total packets recieved: %d", s.totalSent-s.lost)
	log.Printf("Packet loss so far: %f%%", (float64(s.lost)/float64(s.totalSent))*100)
	log.Printf("Average latency: %f ms", s.avgLatency)
}
