package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"github.com/miekg/dns"
)

/*
takes a domain and returns the IP address from the A record
assuming there is one
*/
func GetARecordIP(domain string) (net.IP, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	client := new(dns.Client)
	resp, _, err := client.Exchange(msg, "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	// extract the IP address from the response
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			return net.IP(a.A), nil
		}
	}

	return nil, fmt.Errorf("no A record found for %s", domain)
}

/*
pings an IP address and returns bool if the IP responds or not
*/
func IsIPAlive(ip net.IP) (bool, error) {

	cmd := exec.Command("ping", "-c", "1", "-t", "1", ip.String())
	stdout, err := cmd.Output()

	if err != nil {
		return false, err
	}

	// check the ping response. we want it to fail
	output := string(stdout)
	if strings.Contains(output, "1 packets received") {
		return true, nil
	} else {
		return false, nil
	}

	// default return
	return true, nil
}

/*
parses a slice of EC2 CIDR's
and checks if an IP is within any of the ranges
returns the prefix that the IP is contained in
*/
func IsEC2IPAddress(ip net.IP) (*Prefix, error) {

	for _, prefix := range ec2s {

		_, ipNet, err := net.ParseCIDR(prefix.IPPrefix)
		if err != nil {
			return nil, err
		}
		if ipNet.Contains(ip) {
			return &prefix, nil
		}
	}
	return nil, fmt.Errorf("IP %s not found in an EC2 prefix range\n", ip.String())
}

/*
parses a slice of S3 CIDR's
and checks if an IP is within any of the ranges
returns the prefix that the IP is contained in
*/
func IsS3IPAddress(ip net.IP) (*Prefix, error) {

	for _, prefix := range s3s {

		_, ipNet, err := net.ParseCIDR(prefix.IPPrefix)
		if err != nil {
			return nil, err
		}
		if ipNet.Contains(ip) {
			return &prefix, nil
		}
	}
	return nil, fmt.Errorf("IP %s not found in an S3 prefix range\n", ip.String())
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
parses the latest list of EC2 IP ranges as published by AWS
returns a slice of the Prefix structs which contains the
prefix range, the service and the region
*/
func GetS3IpAddressRanges() ([]Prefix, error) {

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
	var s3Prefixes []Prefix

	for _, prefix := range data.Prefixes {
		if prefix.Service == "S3" {
			s3Prefixes = append(s3Prefixes, prefix)
		}
	}
	return s3Prefixes, nil

}

/*
takes a slice and returns the first element, or not if empty
*/
func hostOrIP(hostnames []string) string {
	if len(hostnames) > 0 {
		return hostnames[0]
	}
	return ""
}

func getStatusCode(uri string) (int, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
