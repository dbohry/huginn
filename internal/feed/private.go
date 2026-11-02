package feed

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrPrivateAddress is returned when a fetch is refused because the address is
// on a private network, and Huginn has been told not to reach those.
var ErrPrivateAddress = errors.New("address is on a private network")

// extraBlocked are ranges net.IP's own checks do not cover.
var extraBlocked = mustCIDRs(
	"0.0.0.0/8",     // "this network"
	"100.64.0.0/10", // carrier-grade NAT, also used by Tailscale
	"192.0.0.0/24",  // IETF protocol assignments
	"198.18.0.0/15", // benchmarking
	"240.0.0.0/4",   // reserved
	"64:ff9b::/96",  // NAT64
)

func mustCIDRs(list ...string) []*net.IPNet {
	var out []*net.IPNet
	for _, c := range list {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

// isPrivate reports whether ip is on this machine or a private, link-local or
// otherwise non-public network.
func isPrivate(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4 // an IPv4 address written in IPv6 form is still an IPv4 address
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range extraBlocked {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// refusePrivate is a net.Dialer Control function. It runs after the name has
// been resolved and just before connecting, so it checks the address that is
// really used. That covers redirects and names that resolve to a private
// address (including ones that change their answer between lookups).
func refusePrivate(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("%w: cannot read %q", ErrPrivateAddress, host)
	}
	if isPrivate(ip) {
		return ErrPrivateAddress
	}
	return nil
}

// BlockPrivateNetworks makes the fetcher refuse to connect to addresses on
// private networks: this machine, home and office networks, link-local and
// similar. Use it when Huginn is shared with people who should not be able to
// probe the network it runs on.
func (f *Fetcher) BlockPrivateNetworks() {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: refusePrivate}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	transport.Proxy = nil // a proxy would be dialled instead of the feed's own address
	f.Client.Transport = transport
}
