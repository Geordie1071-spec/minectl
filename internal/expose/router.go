// Package expose makes a server reachable from the internet: by asking the
// home router to forward its port (UPnP or NAT-PMP), or through a playit.gg
// tunnel when the router can't.
package expose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	"github.com/jackpal/gateway"
	natpmp "github.com/jackpal/go-nat-pmp"
)

const (
	MethodUPnP   = "upnp"
	MethodNATPMP = "nat-pmp"
	MethodPlayit = "playit"
)

// natpmpLifetime is how long a NAT-PMP mapping lasts. The panel renews it well
// before then; NAT-PMP has no permanent mappings.
const natpmpLifetime = 24 * time.Hour

// Mapping is a port forwarded on the router.
type Mapping struct {
	Method     string
	ExternalIP net.IP
	Port       int
}

// Address is what players type to join.
func (m *Mapping) Address() string {
	return net.JoinHostPort(m.ExternalIP.String(), fmt.Sprint(m.Port))
}

// SharedAddressError means the router forwarded the port, but its own
// internet address is private or carrier-grade NAT, so another NAT the user
// doesn't control sits in front of it.
type SharedAddressError struct{ IP net.IP }

func (e *SharedAddressError) Error() string {
	return fmt.Sprintf("your router's internet address (%s) is shared with other customers of your internet provider (CGNAT), so ports can't be forwarded to it", e.IP)
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// Unreachable reports whether ip can't be reached from the internet.
func Unreachable(ip net.IP) bool {
	return ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || cgnat.Contains(ip)
}

type igd interface {
	AddPortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string, internalPort uint16, internalClient string, enabled bool, description string, leaseSeconds uint32) error
	DeletePortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string) error
	GetExternalIPAddressCtx(ctx context.Context) (string, error)
	LocalAddr() net.IP
}

// findIGD looks for a UPnP internet gateway on the local network.
func findIGD(ctx context.Context) (igd, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	type found struct {
		c   igd
		err error
	}
	// Ask for every connection type at once; each search waits for replies.
	searches := []func() found{
		func() found {
			cs, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx)
			if len(cs) > 0 {
				return found{c: cs[0]}
			}
			return found{err: err}
		},
		func() found {
			cs, _, err := internetgateway2.NewWANIPConnection1ClientsCtx(ctx)
			if len(cs) > 0 {
				return found{c: cs[0]}
			}
			return found{err: err}
		},
		func() found {
			cs, _, err := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx)
			if len(cs) > 0 {
				return found{c: cs[0]}
			}
			return found{err: err}
		},
	}
	results := make([]chan found, len(searches))
	for i, search := range searches {
		results[i] = make(chan found, 1)
		go func(ch chan found) { ch <- search() }(results[i])
	}
	var errs []error
	for _, ch := range results {
		r := <-ch
		if r.c != nil {
			return r.c, nil
		}
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return nil, errors.New("no UPnP router found")
}

func mapUPnP(ctx context.Context, port int, description string) (*Mapping, error) {
	c, err := findIGD(ctx)
	if err != nil {
		return nil, err
	}
	local := c.LocalAddr()
	if local == nil {
		return nil, errors.New("couldn't tell which address this machine has on the router's network")
	}
	// Ask for a permanent mapping; the panel re-adds it after router restarts.
	err = c.AddPortMappingCtx(ctx, "", uint16(port), "TCP", uint16(port), local.String(), true, description, 0)
	if err != nil && strings.Contains(err.Error(), "725") {
		// OnlyPermanentLeasesSupported is the reverse case; some routers want a lease.
		err = c.AddPortMappingCtx(ctx, "", uint16(port), "TCP", uint16(port), local.String(), true, description, 3600)
	}
	if err != nil {
		if strings.Contains(err.Error(), "718") {
			return nil, fmt.Errorf("port %d is already forwarded to another device on your network", port)
		}
		return nil, fmt.Errorf("the router refused to forward port %d: %w", port, err)
	}
	ipText, err := c.GetExternalIPAddressCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("the router forwarded the port but didn't say its internet address: %w", err)
	}
	return &Mapping{Method: MethodUPnP, ExternalIP: net.ParseIP(strings.TrimSpace(ipText)), Port: port}, nil
}

func mapNATPMP(port int) (*Mapping, error) {
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		return nil, fmt.Errorf("couldn't find the router: %w", err)
	}
	c := natpmp.NewClientWithTimeout(gw, 3*time.Second)
	res, err := c.AddPortMapping("tcp", port, port, int(natpmpLifetime.Seconds()))
	if err != nil {
		return nil, err
	}
	if int(res.MappedExternalPort) != port {
		_, _ = c.AddPortMapping("tcp", port, int(res.MappedExternalPort), 0)
		return nil, fmt.Errorf("the router would only forward port %d, not %d", res.MappedExternalPort, port)
	}
	ext, err := c.GetExternalAddress()
	if err != nil {
		return nil, err
	}
	ip := ext.ExternalIPAddress
	return &Mapping{Method: MethodNATPMP, ExternalIP: net.IPv4(ip[0], ip[1], ip[2], ip[3]), Port: port}, nil
}

// MapPort asks the router to forward TCP port to this machine, trying UPnP and
// then NAT-PMP. If prefer is set, that method is tried first, so renewals stay
// on the method that worked.
func MapPort(ctx context.Context, port int, description, prefer string) (*Mapping, error) {
	tries := []string{MethodUPnP, MethodNATPMP}
	if prefer == MethodNATPMP {
		tries = []string{MethodNATPMP, MethodUPnP}
	}
	var errs []string
	for _, method := range tries {
		var m *Mapping
		var err error
		if method == MethodUPnP {
			m, err = mapUPnP(ctx, port, description)
		} else {
			m, err = mapNATPMP(port)
		}
		if err != nil {
			errs = append(errs, method+": "+err.Error())
			continue
		}
		if Unreachable(m.ExternalIP) {
			_ = UnmapPort(ctx, m.Method, port)
			return nil, &SharedAddressError{IP: m.ExternalIP}
		}
		return m, nil
	}
	return nil, fmt.Errorf("your router doesn't allow automatic port forwarding (UPnP and NAT-PMP are off or unsupported)\n  %s", strings.Join(errs, "\n  "))
}

// UnmapPort removes a forward made by MapPort.
func UnmapPort(ctx context.Context, method string, port int) error {
	switch method {
	case MethodUPnP:
		c, err := findIGD(ctx)
		if err != nil {
			return err
		}
		return c.DeletePortMappingCtx(ctx, "", uint16(port), "TCP")
	case MethodNATPMP:
		gw, err := gateway.DiscoverGateway()
		if err != nil {
			return err
		}
		_, err = natpmp.NewClientWithTimeout(gw, 3*time.Second).AddPortMapping("tcp", port, port, 0)
		return err
	}
	return nil
}

// Hosted reports whether this machine is on the internet directly, as a VPS
// usually is, and its public IP: servers are then reachable without any port
// forwarding. That's either an address of its own, or on AWS, Google Cloud,
// Azure and Oracle, the one the provider forwards to its private IP.
func Hosted() (net.IP, bool) {
	ip := net.ParseIP(LANIP())
	if ip != nil && !Unreachable(ip) {
		return ip, true
	}
	if c := DetectCloud(); c != nil && c.IP != nil {
		return c.IP, true
	}
	return ip, false
}

// LANIP is this machine's address on the local network, for players at home.
func LANIP() string {
	conn, err := net.Dial("udp", "192.0.2.1:9")
	if err == nil {
		defer conn.Close()
		if a, ok := conn.LocalAddr().(*net.UDPAddr); ok && !a.IP.IsLoopback() {
			return a.IP.String()
		}
	}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if n, ok := addr.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
			return n.IP.String()
		}
	}
	return ""
}
