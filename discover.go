package main

import (
	"context"
	"log"
	"net"

	"github.com/miekg/dns"
)

var (
	ipv4mcastdns *net.UDPAddr
	ipv6mcastdns *net.UDPAddr
)

type response struct {
	ServiceName string
	DeviceName  string
	IP          net.IP
	Port        uint16
}

func init() {
	ipv4mcastdns, _ = net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	ipv6mcastdns, _ = net.ResolveUDPAddr("udp", "ff02::fb:5353")
}

func Discover(serviceName string) (<-chan *response, context.CancelFunc) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	msgChan := make(chan *response)

	go handler(ctx, msgChan)
	go query(serviceName)
	return msgChan, cancel
}

func query(serviceName string) error {
	var msg dns.Msg
	msg.SetQuestion(serviceName, dns.TypePTR)

	b, err := msg.Pack()
	if err != nil {
		return err
	}

	conn, err := net.Dial("udp", "224.0.0.251:5353")
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func handler(ctx context.Context, out chan<- *response) error {
	conn, err := net.ListenMulticastUDP("udp", nil, ipv4mcastdns)
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, 1500)

	var msg dns.Msg
	for {
		select {
		case <-ctx.Done():
			log.Println("canceled")
			break
		default:
			read, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			err = msg.Unpack(buf[:read])
			if err != nil {
				continue
			}

			if !msg.MsgHdr.Response {
				continue
			}

			out <- decode(&msg)
		}
	}
}

func decode(msg *dns.Msg) *response {
	// RFC 6763 section 12.1 - 12.4
	ans := msg.Answer[0].(*dns.PTR)
	serviceName := ans.Hdr.Name
	deviceName := ans.Ptr
	deviceName = deviceName[:len(deviceName)-len(serviceName)-1]
	var ip net.IP
	var port uint16

	for _, rr := range msg.Extra {
		switch rr := rr.(type) {
		case *dns.SRV:
			port = rr.Port
		case *dns.A:
			ip = rr.A
		case *dns.TXT:
			// TODO! Parse keys and values defined in RFC 6763 section 6
			continue
		default:
			continue
		}
	}

	return &response{
		ServiceName: serviceName,
		DeviceName:  deviceName,
		IP:          ip,
		Port:        port,
	}
}
