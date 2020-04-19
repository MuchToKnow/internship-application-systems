# Preface
Hi there!  Thanks for taking a look at my project!  I have been wanting to learn go and I decided this was a great opportunity and project to jump into it with.  Given this, please excuse any antipatterns or code smells I might have let slip by.

# Regarding the project
I was not able to get non-priviledged ICMP packets working on my machine (see: https://godoc.org/golang.org/x/net/icmp#PacketConn) so I had to do the project assuming priveledged mode (sudo).

# Running the project
Ensure you have installed:
- "golang.org/x/net/icmp"
-	"golang.org/x/net/ipv4"
- "golang.org/x/net/ipv6"

And `go` itself of course!

In the src directory:

`go build main.go`

`sudo ./main google.com`

and you should be pinging google with RTT, Average latency and more being reported.

General usage:

`sudo ./main <domain or ip>`

OR

`sudo ./main <domain or ip> -c <# of pings>`

# Extra features
I have implemented support for ipv6 but can not test it as my ISP does not seem to support ipv6 at the moment according to https://test-ipv6.com/.  Even the ping command is not working with ipv6 addresses on my machine.  My program currently reports `sendto: network is unreachable` for these addresses so I would love if whoever takes a look at this can test out my ipv6 support!

I also implemented the -c flag from the ping man page if you would like the program to terminate after a certain # of pings.

# Screenshots
![Hostname Resolution](/screenshots/HostnameResolution.png?raw=true "Hostname Resolution")

![Packet Loss Example](/screenshots/PacketLossExample.png?raw=true "Example with some loss")

![Pinging My Public IP](/screenshots/PingingMyself.png?raw=true "Pinging a raw ipv4 (my public ip)")

![-c usage](/screenshots/UsageWith-c.png?raw=true "Example of -c usage")

