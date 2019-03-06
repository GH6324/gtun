package gtund

import (
	"bufio"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/ICKelin/gtun/logs"
)

type ReverseConfig struct {
	ruleFile string
}

type Reverse struct {
	sync.Mutex
	ruleFile string
	policy   []*ReversePolicy
}

func NewReverse(cfg *ReverseConfig) (*Reverse, error) {
	reverse := &Reverse{
		ruleFile: cfg.ruleFile,
	}

	policy, err := LoadReversePolicy(cfg.ruleFile)
	if err != nil {
		return nil, err
	}
	reverse.policy = policy

	for _, r := range reverse.policy {
		go Proxy(r.Proto, r.From, r.To)
	}

	return reverse, nil
}

type ReversePolicy struct {
	Proto string `json:"proto"`
	From  string `json:"from"`
	To    string `to:"json"`
}

func LoadReversePolicy(path string) ([]*ReversePolicy, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reverse := make([]*ReversePolicy, 0)

	reader := bufio.NewReader(fp)
	for {
		bline, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}

		line := string(bline)
		if len(line) > 0 && line[0] == '#' {
			continue
		}

		sp1 := strings.Split(line, " ")
		if len(sp1) != 2 {
			continue
		}

		sp2 := strings.Split(sp1[1], "->")
		if len(sp2) != 2 {
			continue
		}

		reversePolicy := &ReversePolicy{
			Proto: sp1[0],
			From:  sp2[0],
			To:    sp2[1],
		}

		reverse = append(reverse, reversePolicy)
	}

	return reverse, nil
}

func Proxy(prot string, from, to string) {
	if strings.ToLower(prot) == "tcp" {
		ProxyTCP(from, to)
	}

	if strings.ToLower(prot) == "udp" {
		ProxyUDP(from, to)
	}
}

func ProxyTCP(from, to string) {
	listener, err := net.Listen("tcp", from)
	if err != nil {
		logs.Error("listen fail: %v", err)
		return
	}

	logs.Info("proxy pass tcp %s => %s", from, to)
	for {
		conn, err := listener.Accept()
		if err != nil {
			logs.Error("accept: %v", err)
			break
		}

		go reverse("tcp", conn, to)
	}
}

func ProxyUDP(from, to string) {
	laddr, err := net.ResolveUDPAddr("udp", from)
	if err != nil {
		logs.Error("resolve udp fail: %v", err)
		return
	}

	lconn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		logs.Error("listen udp fail: %v", err)
		return
	}

	logs.Info("proxy pass udp %s => %s", from, to)
	go reverse("udp", lconn, to)
}

func reverse(proto string, clientconn net.Conn, to string) {
	defer clientconn.Close()
	rconn, err := net.Dial(proto, to)
	if err != nil {
		logs.Error("dial: %v", err)
		return
	}
	defer rconn.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// client->rconn
	go func() {
		defer wg.Done()
		Copy(rconn, clientconn)
	}()

	// rconn->client
	go func() {
		defer wg.Done()
		Copy(clientconn, rconn)
	}()

	wg.Wait()
}

func Copy(dst, src net.Conn) {
	buffer := make([]byte, 2048)
	for {
		nr, err := src.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logs.Error("read fail: %v", err)
			}
			break
		}

		_, err = dst.Write(buffer[:nr])
		if err != nil {
			logs.Error("write to peer fail: %v", err)
			break
		}
	}

}
