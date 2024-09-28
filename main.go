package main

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	FullChain string = "fullchain.pem"
	PrivKey   string = "privkey.pem"
)

const HetzBotLabel string = "hetzbot"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	_ = godotenv.Load()

	lineage := os.Getenv("RENEWED_LINEAGE")
	if lineage == "" {
		slog.Error("RENEWED_LINEAGE is missing")
		os.Exit(1)
	}

	renewed := strings.Split(os.Getenv("RENEWED_DOMAINS"), " ")
	if len(renewed) == 0 {
		log.Error("RENEWED_DOMAINS is empty")
		os.Exit(1)
	}

	token := os.Getenv("HETZNER_TOKEN")
	if token == "" {
		log.Error("HETZNER_TOKEN is missing")
		os.Exit(1)
	}

	lbID, err := strconv.Atoi(strings.TrimSpace(os.Getenv("HETZNER_LB_ID")))
	if err != nil {
		log.Error("HETZNER_LB_ID is invalid", "error", err)
		os.Exit(1)
	}

	certName := fmt.Sprintf("%s-%d", path.Base(lineage), time.Now().Unix())

	client := hcloud.NewClient(hcloud.WithToken(token))

	ctx := context.Background()
	balancer, _, err := client.LoadBalancer.GetByID(ctx, int64(lbID))
	if err != nil || balancer == nil {
		log.Error("could not get load balancer", "error", err)
		os.Exit(1)
	}

	var service hcloud.LoadBalancerService
	var oldCert *hcloud.Certificate

	for _, svc := range balancer.Services {
		if svc.Protocol != hcloud.LoadBalancerServiceProtocolHTTPS {
			continue
		}

		service = svc

		for _, c := range svc.HTTP.Certificates {
			cert, _, err := client.Certificate.GetByID(ctx, c.ID)
			if cert == nil || err != nil {
				log.Error("could not get certificate",
					"certificate_id", c.ID,
					"error", err,
				)
				os.Exit(1)
			}

			if !sameSlice(renewed, cert.DomainNames) {
				continue
			}

			oldCert = cert
		}
	}

	fullchain, err := os.ReadFile(fmt.Sprintf("%s/%s", lineage, FullChain))
	if err != nil {
		log.Error("could not read fullchain.pem", "error", err)
		os.Exit(1)
	}
	privkey, err := os.ReadFile(fmt.Sprintf("%s/%s", lineage, PrivKey))
	if err != nil {
		log.Error("could not read privkey.pem", "error", err)
		os.Exit(1)
	}

	newCert, _, err := client.Certificate.Create(ctx, hcloud.CertificateCreateOpts{
		Name:        certName,
		Type:        hcloud.CertificateTypeUploaded,
		Certificate: string(fullchain),
		PrivateKey:  string(privkey),
		Labels:      map[string]string{HetzBotLabel: "true"},
		DomainNames: renewed,
	})
	if err != nil {
		log.Error("could not create certificate", "error", err)
		os.Exit(1)
	}

	service.HTTP.Certificates = append(service.HTTP.Certificates, newCert)
	action, _, err := client.LoadBalancer.UpdateService(ctx, balancer, service.ListenPort, hcloud.LoadBalancerUpdateServiceOpts{
		Protocol: service.Protocol,
		HTTP: &hcloud.LoadBalancerUpdateServiceOptsHTTP{
			Certificates: service.HTTP.Certificates,
		},
	})
	if err != nil ||
		action.Status != hcloud.ActionStatusSuccess ||
		action.ErrorCode != "" ||
		action.Progress != 100 {
		log.Error("could not update load balancer service with new certificate",
			"action", action,
			"error", err)
		os.Exit(1)
	}

	if oldCert == nil {
		return
	}

	newCerts := make([]*hcloud.Certificate, 0, len(service.HTTP.Certificates))
	for _, cert := range service.HTTP.Certificates {
		if cert.ID == oldCert.ID {
			continue
		}

		newCerts = append(newCerts, cert)
	}

	action, _, err = client.LoadBalancer.UpdateService(ctx, balancer, service.ListenPort, hcloud.LoadBalancerUpdateServiceOpts{
		Protocol: service.Protocol,
		HTTP: &hcloud.LoadBalancerUpdateServiceOptsHTTP{
			Certificates: newCerts,
		},
	})
	if err != nil ||
		action.Status != hcloud.ActionStatusSuccess ||
		action.ErrorCode != "" ||
		action.Progress != 100 {
		log.Error("could not update load balancer service with old certificate removed",
			"action", action,
			"error", err,
		)
		os.Exit(1)
	}
}

func sameSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	diff := make(map[string]int, len(a))
	for _, d := range a {
		diff[d]++
	}
	for _, d := range b {
		if _, ok := diff[d]; !ok {
			return false
		}
		diff[d]--
		if diff[d] == 0 {
			delete(diff, d)
		}
	}

	return len(diff) == 0
}
