package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gookit/color"
	"github.com/miekg/dns"
)

// globals
var concurrency int
var to int
var ec2s []Prefix

// channels
var domains = make(chan string, 200)

// types
type Prefix struct {
	IPPrefix           string `json:"ip_prefix"`
	Region             string `json:"region"`
	Service            string `json:"service"`
	NetworkBorderGroup string `json:"network_border_group"`
}

type Response struct {
	SyncToken  string   `json:"syncToken"`
	CreateDate string   `json:"createDate"`
	Prefixes   []Prefix `json:"prefixes"`
}

/*
ensures the current ec2 cidr ranges can be obtained
*/
func init() {
	var err error
	ec2s, err = GetEc2IpAddressRanges()
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")
	flag.IntVar(&to, "t", 5000, "timeout (milliseconds)")

	flag.Parse()

	/*
		http client logic taken from httprobe
	*/
	timeout := time.Duration(to * 1000000)

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// client is then passed in the goroutine
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	// wait group
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range domains {

				// 1. perform a GET to see if it's alive. skip if so.
				withProto := "http://" + domain
				if isListening(client, withProto) {
					continue
				}

				// 2. get the A record and see if it's dead
				ips, err := GetARecordIP(domain)
				if err != nil {
					log.Fatalln(err)
				}

				// see if any IPs in the A record are dead
				for _, ip := range ips {

					// ignore private IPs
					if ip.IsPrivate() {
						continue
					}

					// ignore case of A record being blank
					if ip.Equal(net.ParseIP("0.0.0.0")) {
						continue
					}

					// check if dead
					dead := IsDead(ip)
					if dead {

						// highlight any dead EC2 IPs
						ok, prefix := IsEC2IPAddress(ip)
						if ok {
							color.Green.Printf("%s,%s,%s\n", domain, ip, prefix.Region)
						} else {
							fmt.Printf("%s,%s\n", domain, ip)
						}
					}
				}

			}
		}()
	}

	// this sends to the jobs channel
	_, err := GetUserInput()
	if err != nil {
		log.Fatalln("Failed to fetch user input, please retry.")
	}

	// tidy up
	close(domains)
	wg.Wait()

}

/*
get a list from the user and send to the channel to work
*/
func GetUserInput() (bool, error) {

	seen := make(map[string]bool)

	// read from stdin or from arg
	var input_lines io.Reader
	input_lines = os.Stdin

	arg_line := flag.Arg(0)
	if arg_line != "" {
		input_lines = strings.NewReader(arg_line)
	}

	sc := bufio.NewScanner(input_lines)

	for sc.Scan() {

		line := strings.ToLower(sc.Text())

		// ignore domains we've seen
		if _, ok := seen[line]; ok {
			continue
		}

		seen[line] = true

		domains <- line

	}

	// check there were no errors reading stdin
	if err := sc.Err(); err != nil {
		return false, err
	}

	return true, nil
}

/*
pings an IP address and returns true if IP does not respond
*/
func IsDead(ip net.IP) bool {

	cmd := exec.Command("ping", "-c", "1", "-t", "1", ip.String())
	stdout, err := cmd.Output()

	if err != nil {
		return true
	}

	// check the ping response. we want it to fail
	output := string(stdout)
	if strings.Contains(output, "1 packets received") {
		return false
	} else {
		return true
	}

	return false
}

/*
takes a domain and returns all IPs from the A record
assuming there are any
*/
func GetARecordIP(domain string) ([]net.IP, error) {

	// slice of IPs
	var out []net.IP

	// set the question
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	client := new(dns.Client)

	// ask
	resp, _, err := client.Exchange(msg, "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	// extract each IP from the answer
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			out = append(out, a.A)
		}
	}

	// returns slice
	return out, nil

}

/*
determines if a url is listening on HTTP
taken from github.com/tomnomnom/httprobe
*/
func isListening(client *http.Client, url string) bool {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)
	if resp != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil {
		return false
	}

	return true
}

/*
parses the latest list of EC2 IP ranges as published by AWS
returns a slice of the Prefix structs which contains the
prefix range, the service and the region
*/
func GetEc2IpAddressRanges() ([]Prefix, error) {

	url := "https://ip-ranges.amazonaws.com/ip-ranges.json"

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var data Response
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	// filter out the ip_prefixes for EC2 service
	var ec2Prefixes []Prefix

	for _, prefix := range data.Prefixes {
		if prefix.Service == "EC2" {
			ec2Prefixes = append(ec2Prefixes, prefix)
		}
	}
	return ec2Prefixes, nil

}

/*
parses a slice of EC2 CIDR's
and checks if an IP is within any of the ranges
returns the prefix that the IP is contained in
*/
func IsEC2IPAddress(ip net.IP) (bool, *Prefix) {

	for _, prefix := range ec2s {

		_, ipNet, err := net.ParseCIDR(prefix.IPPrefix)
		if err != nil {
			return false, nil
		}
		if ipNet.Contains(ip) {
			return true, &prefix
		}
	}
	return false, nil
}
