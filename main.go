/*
bucketeer

takes a list of domains piped from stdin
looks up the A record for each
if the A record is an IP address, checks if
if the IP is dead, it prints
also highlights if the IP is an EC2 host

the aim is to provide a list of IP addresses that may be vulnerable to a takeover
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/gookit/color"
)

var concurrency int
var domains = make(chan string, 1)
var jobs = make(chan *Job)
var ec2s []Prefix
var s3s []Prefix

/*
prints the usage if no input is provided to the program
*/
func print_usage() {
	log.Fatalln("Expected usage: cat <domains> | ipwzrd")
}

/*
ensures the current ec2 cidr ranges can be obtained
*/
func init() {
	var err error

	// check if the Go version is supported
	if !strings.HasPrefix(runtime.Version(), "go1.19") {
		log.Fatalln("This program requires Go version 1.19 or higher.")
	}

	ec2s, err = GetEc2IpAddressRanges()
	if err != nil {
		log.Fatalln(err)
	}

	s3s, err = GetS3IpAddressRanges()
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")
	flag.Parse()

	/*
		iterate the domains channel
		get the A record IP address
		send to the jobs channel
	*/
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for domain := range domains {

				ip, err := GetARecordIP(domain)
				if err != nil {
					continue
				}

				job := &Job{}
				job.domain = domain
				job.ip = ip

				jobs <- job

			}
		}()
	}

	/*
		receives a job
		checks if the IP address is alive
		checks if the IP address is in an EC2 CIDR
		prints output
	*/
	var jg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		jg.Add(1)
		go func() {
			defer jg.Done()

			for job := range jobs {

				// an error is actually what we want, hence ignoring err
				// the func will return false if there's an err
				alive, _ := IsIPAlive(job.ip)

				if !alive {
					host, _ := net.LookupAddr(job.ip.String())
					ec2, _ := IsEC2IPAddress(job.ip)

					if ec2 != nil {
						color.Green.Printf("%s,%s,%s\n", job.domain, hostOrIP(host), job.ip.String())
					} else {
						fmt.Printf("%s,%s\n", job.domain, strings.Join([]string{hostOrIP(host), job.ip.String()}, ","))
					}
				}

				// check if the A record points to an S3 bucket
				s3, _ := IsS3IPAddress(job.ip)

				// generate the S3 URI
				if s3 != nil {
					s3uri := fmt.Sprintf("http://%s.s3-website-%s.amazonaws.com", job.domain, s3.Region)
					status, err := getStatusCode(s3uri)
					if err != nil {
						log.Println(err)
					}
					if status != 200 {
						color.Green.Printf("%s,%s\n", job.domain, s3uri)
					}
				}
				// http://[domain].s3-website-[region].amazonaws.com
				// http://secret.velvetsweat.shop.s3-website-us-east-1.amazonaws.com
			}
		}()
	}

	// check for input piped to stdin
	info, err := os.Stdin.Stat()
	if err != nil {
		log.Fatal(err)
	}
	if info.Mode()&os.ModeCharDevice != 0 || info.Size() <= 0 {
		print_usage()
	}

	// get user input
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		domains <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		log.Println(err)
	}

	// tidy up
	close(domains)
	wg.Wait()

	close(jobs)
	jg.Wait()

}
