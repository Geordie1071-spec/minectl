package expose

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Cloud is the provider a VPS runs on. On AWS, Google Cloud, Azure and Oracle
// the machine only sees a private IP: the public one lives on the provider's
// side, so it's looked up from the provider's metadata service.
type Cloud struct {
	// Provider is a key of providers, e.g. "aws".
	Provider string
	// Name is how the provider calls itself, for messages.
	Name string
	// IP is the machine's public address, nil when it has none.
	IP net.IP
}

// provider is what minectl recommends on each cloud. minectl never touches
// the provider's console itself: it says where to go instead.
type provider struct {
	name string
	// firewall is where to open ports in the provider's console.
	firewall string
	// tip is the one other thing worth doing there, if any.
	tip string
	// privateIP marks clouds where the machine only sees a private IP.
	privateIP bool
}

var providers = map[string]provider{
	"aws": {
		name:      "AWS",
		firewall:  "the instance's security group (EC2 › Instances › Security › the security group › Edit inbound rules, source 0.0.0.0/0)",
		tip:       "Attach an Elastic IP (EC2 › Elastic IPs › Allocate, then Associate) so the address stays the same when the instance is stopped and started.",
		privateIP: true,
	},
	"gcp": {
		name:      "Google Cloud",
		firewall:  "a VPC firewall rule (VPC network › Firewall › Create firewall rule, targets all instances, source 0.0.0.0/0)",
		tip:       "Make the external IP static (VPC network › IP addresses › the VM's address › Promote to static) so it survives a stop and start.",
		privateIP: true,
	},
	"azure": {
		name:      "Azure",
		firewall:  "the VM's network security group (the VM › Networking › Add inbound port rule)",
		privateIP: true,
	},
	"oracle": {
		name:      "Oracle Cloud",
		firewall:  "the subnet's security list (Networking › Virtual cloud networks › your VCN › Security Lists › Add Ingress Rules, source 0.0.0.0/0)",
		privateIP: true,
	},
	"hetzner": {
		name:     "Hetzner",
		firewall: "your Hetzner Cloud Firewall if you added one (Firewalls in the Cloud Console)",
	},
	"digitalocean": {
		name:     "DigitalOcean",
		firewall: "your DigitalOcean Cloud Firewall if you added one (Networking › Firewalls)",
	},
	"vultr": {
		name:     "Vultr",
		firewall: "your Vultr firewall group if you added one (Network › Firewall)",
	},
}

// Firewall says where in the provider's console to open TCP ports, e.g.
// "25565" or "80 and 443". A nil Cloud gives the generic advice.
func (c *Cloud) Firewall(ports string) string {
	if c == nil {
		return "open TCP " + ports + " in your cloud provider's firewall"
	}
	return "open TCP " + ports + " in " + providers[c.Provider].firewall
}

// Tip is one more recommendation for the provider's console, or empty.
func (c *Cloud) Tip() string {
	if c == nil {
		return ""
	}
	return providers[c.Provider].tip
}

// dmiDir is where Linux shows the machine's firmware details. Replaced in tests.
var dmiDir = "/sys/class/dmi/id"

// cloudFromDMI recognises clouds from the firmware details, which are
// readable without root and cost nothing, so home machines never wait on a
// metadata request.
func cloudFromDMI() string {
	read := func(name string) string {
		b, _ := os.ReadFile(dmiDir + "/" + name)
		return strings.TrimSpace(string(b))
	}
	vendor, product, tag, bios := read("sys_vendor"), read("product_name"), read("chassis_asset_tag"), read("bios_version")
	switch {
	case vendor == "Amazon EC2" || strings.Contains(strings.ToLower(bios), "amazon"):
		return "aws"
	case product == "Google Compute Engine":
		return "gcp"
	case tag == "7783-7084-3265-9085": // Azure's fixed asset tag
		return "azure"
	case tag == "OracleCloud.com":
		return "oracle"
	case vendor == "Hetzner":
		return "hetzner"
	case vendor == "DigitalOcean":
		return "digitalocean"
	case vendor == "Vultr":
		return "vultr"
	}
	return ""
}

// Replaced in tests.
var (
	metadataBase = "http://169.254.169.254"
	ipEchoURL    = "https://checkip.amazonaws.com"
)

// metadataClient never goes through a proxy: the metadata service is only
// reachable from the machine itself.
var metadataClient = &http.Client{
	Timeout:   2 * time.Second,
	Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext},
}

func get(ctx context.Context, c *http.Client, method, url string, header map[string]string) (string, http.Header, bool) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", nil, false
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", nil, false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return strings.TrimSpace(string(b)), resp.Header, resp.StatusCode == http.StatusOK
}

// metadataIP asks the provider's metadata service for the public IP. Oracle's
// doesn't know it, and Azure's often doesn't.
func metadataIP(ctx context.Context, provider string) net.IP {
	var text string
	switch provider {
	case "aws":
		// IMDSv2: a session token first; newer instances require it.
		token, _, ok := get(ctx, metadataClient, http.MethodPut, metadataBase+"/latest/api/token", map[string]string{"X-aws-ec2-metadata-token-ttl-seconds": "60"})
		if !ok {
			return nil
		}
		text, _, _ = get(ctx, metadataClient, http.MethodGet, metadataBase+"/latest/meta-data/public-ipv4", map[string]string{"X-aws-ec2-metadata-token": token})
	case "gcp":
		var h http.Header
		text, h, _ = get(ctx, metadataClient, http.MethodGet, metadataBase+"/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip", map[string]string{"Metadata-Flavor": "Google"})
		if h.Get("Metadata-Flavor") != "Google" {
			return nil
		}
	case "azure":
		text, _, _ = get(ctx, metadataClient, http.MethodGet, metadataBase+"/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text", map[string]string{"Metadata": "true"})
	}
	if ip := net.ParseIP(text); ip != nil && !Unreachable(ip) {
		return ip
	}
	return nil
}

// echoIP asks a public service which address this machine's traffic comes
// from. On a cloud VM that's its public IP, forwarded one to one.
func echoIP(ctx context.Context) net.IP {
	text, _, ok := get(ctx, http.DefaultClient, http.MethodGet, ipEchoURL, nil)
	if ip := net.ParseIP(text); ok && ip != nil && !Unreachable(ip) {
		return ip
	}
	return nil
}

func detectCloud(ctx context.Context) *Cloud {
	provider := cloudFromDMI()
	if provider == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c := &Cloud{Provider: provider, Name: providers[provider].name}
	if !providers[provider].privateIP {
		// The machine has its public IP itself.
		if ip := net.ParseIP(LANIP()); !Unreachable(ip) {
			c.IP = ip
		}
		return c
	}
	c.IP = metadataIP(ctx, provider)
	// AWS and Google always know; asking elsewhere there would find a NAT
	// gateway's address on a machine with no public IP of its own.
	if c.IP == nil && (provider == "oracle" || provider == "azure") {
		c.IP = echoIP(ctx)
	}
	return c
}

// cloudTTL is how long a lookup is reused. A public IP changes when an
// instance without a reserved one is stopped and started again.
const cloudTTL = 10 * time.Minute

var cloudCache struct {
	sync.Mutex
	c  *Cloud
	at time.Time
}

// DetectCloud reports which cloud this machine runs on, and its public IP.
// It's nil anywhere else, including at home.
func DetectCloud() *Cloud {
	cloudCache.Lock()
	defer cloudCache.Unlock()
	if !cloudCache.at.IsZero() && time.Since(cloudCache.at) < cloudTTL {
		return cloudCache.c
	}
	cloudCache.c = detectCloud(context.Background())
	cloudCache.at = time.Now()
	return cloudCache.c
}
