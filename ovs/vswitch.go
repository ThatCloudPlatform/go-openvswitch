// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	// DefaultIngressRatePolicing is used to disable the ingress policing,
	// which is the default behavior.
	DefaultIngressRatePolicing = int64(-1)

	// DefaultIngressBurstPolicing is to change the ingress policing
	// burst to the default size, 1000 kb.
	DefaultIngressBurstPolicing = int64(-1)
)

// A VSwitchService is used in a Client to execute 'ovs-vsctl' commands.
type VSwitchService struct {
	// Get wraps functionality of the 'ovs-vsctl get' subcommand.
	Get *VSwitchGetService

	// Set wraps functionality of the 'ovs-vsctl set' subcommand.
	Set *VSwitchSetService

	// Wrapped Client for ExecFunc and debugging.
	c *Client
}

// AddBridge attaches a bridge to Open vSwitch.  The bridge may or may
// not already exist.
func (v *VSwitchService) AddBridge(bridge string) error {
	_, err := v.exec("--may-exist", "add-br", bridge)
	return err
}
// AddBond attaches a bond to a bridge on Open vSwitch.  The ports may or may
// not already exist.
func (v *VSwitchService) AddBond(bridge string, bond string, ports ...string) error {
	args := []string{"--may-exist", "add-bond", bridge, bond}
	args = append(args, ports...)
	_, err := v.exec(args...)
	return err
}

// AddBondWithOptions attaches a bond to a bridge on Open vSwitch.  The ports may or may
// not already exist. extraArgs is used to pass additional arguments,
// like "-- set interface <port> type=dpdk options:dpdk-devargs=<pci-address>"
func (v *VSwitchService) AddBondWithOptions(bridge string, bond string, options BondOptions) error {
	args := []string{"--may-exist", "add-bond", bridge, bond}

	// pass additional arguments
	args = append(args, options.slice()...)

	_, err := v.exec(args...)
	return err
}

// AddPort attaches a port to a bridge on Open vSwitch.  The port may or may
// not already exist.
func (v *VSwitchService) AddPort(bridge string, port string) error {
	_, err := v.exec("--may-exist", "add-port", bridge, string(port))
	return err
}

func (v *VSwitchService) AddPortWithOptions(bridge string, port string, options InterfaceOptions) error {
	args := []string{"--may-exist", "add-port", bridge, port}
	args = append(args, options.slice()...)
	_, err := v.exec(args...)
	return err
}

// DeleteBridge detaches a bridge from Open vSwitch.  The bridge may or may
// not already exist.
func (v *VSwitchService) DeleteBridge(bridge string) error {
	_, err := v.exec("--if-exists", "del-br", bridge)
	return err
}

// DeletePort detaches a port from a bridge on Open vSwitch.  The port may or may
// not already exist.
func (v *VSwitchService) DeletePort(bridge string, port string) error {
	_, err := v.exec("--if-exists", "del-port", bridge, string(port))
	return err
}

// ListPorts lists the ports in Open vSwitch.
func (v *VSwitchService) ListPorts(bridge string) ([]string, error) {
	output, err := v.exec("list-ports", bridge)
	if err != nil {
		return nil, err
	}

	// Do no ports exist?
	if len(output) == 0 {
		return nil, nil
	}

	ports := strings.Split(strings.TrimSpace(string(output)), "\n")
	return ports, nil
}

// ListBridges lists the bridges in Open vSwitch.
func (v *VSwitchService) ListBridges() ([]string, error) {
	output, err := v.exec("list-br")
	if err != nil {
		return nil, err
	}

	// Do no bridges exist?
	if len(output) == 0 {
		return nil, nil
	}

	bridges := strings.Split(strings.TrimSpace(string(output)), "\n")
	return bridges, nil
}

// PortToBridge attempts to determine which bridge a port is attached to.
// If port does not exist, an error will be returned, which can be checked
// using IsPortNotExist.
func (v *VSwitchService) PortToBridge(port string) (string, error) {
	out, err := v.exec("port-to-br", string(port))
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// GetFailMode gets the FailMode for the specified bridge.
func (v *VSwitchService) GetFailMode(bridge string) (FailMode, error) {
	out, err := v.exec("get-fail-mode", bridge)
	if err != nil {
		return "", err
	}

	return FailMode(out), nil
}

// SetFailMode sets the specified FailMode for the specified bridge.
func (v *VSwitchService) SetFailMode(bridge string, mode FailMode) error {
	_, err := v.exec("set-fail-mode", bridge, string(mode))
	return err
}

// SetController sets the controller for this bridge so that ovs-ofctl
// can use this address to communicate.
func (v *VSwitchService) SetController(bridge string, address string) error {
	_, err := v.exec("set-controller", bridge, address)
	return err
}

// GetController gets the controller address for this bridge.
func (v *VSwitchService) GetController(bridge string) (string, error) {
	address, err := v.exec("get-controller", bridge)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(address)), nil
}

// exec executes an ExecFunc using 'ovs-vsctl'.
func (v *VSwitchService) exec(args ...string) ([]byte, error) {
	return v.c.exec("ovs-vsctl", args...)
}

// A VSwitchGetService is used in a VSwitchService to execute 'ovs-vsctl get'
// subcommands.
type VSwitchGetService struct {
	// v provides the required exec method.
	v *VSwitchService
}

// Bridge gets configuration for a bridge and returns the values through
// a BridgeOptions struct.
func (v *VSwitchGetService) Bridge(bridge string) (BridgeOptions, error) {
	// We only support the protocol option at this point.
	args := []string{"--format=json", "get", "bridge", bridge, "protocols"}
	out, err := v.v.exec(args...)
	if err != nil {
		return BridgeOptions{}, err
	}

	var protocols []string
	if err := json.Unmarshal(out, &protocols); err != nil {
		return BridgeOptions{}, err
	}

	return BridgeOptions{
		Protocols: protocols,
	}, nil
}

// A VSwitchSetService is used in a VSwitchService to execute 'ovs-vsctl set'
// subcommands.
type VSwitchSetService struct {
	// v provides the required exec method.
	v *VSwitchService
}

// Bridge sets configuration for a bridge using the values from a BridgeOptions
// struct.
func (v *VSwitchSetService) Bridge(bridge string, options BridgeOptions) error {
	// Prepend command line arguments before expanding options slice
	// and appending it
	args := []string{"set", "bridge", bridge}
	args = append(args, options.slice()...)

	_, err := v.v.exec(args...)
	return err
}

// An BridgeOptions enables configuration of a bridge.
type BridgeOptions struct {
	// Protocols specifies the OpenFlow protocols the bridge should use.
	Protocols []string
}

// slice creates a string slice containing any non-zero option values from the
// struct in the format expected by Open vSwitch.
func (o BridgeOptions) slice() []string {
	var s []string

	if len(o.Protocols) > 0 {
		s = append(s, fmt.Sprintf("protocols=%s", strings.Join(o.Protocols, ",")))
	}

	return s
}

// Bond sets configuration for a bond using the values from a BondOptions
// struct.
func (v *VSwitchSetService) Bond(bond string, options BondOptions) error {
	// Prepend command line arguments before expanding options slice
	// and appending it
	args := []string{"set", "bond", bond}
	args = append(args, options.slice()...)

	_, err := v.v.exec(args...)
	return err
}

// A BondOptions struct enables configuration of a Bond.
type BondOptions struct {
	// See: https://docs.redhat.com/en/documentation/red_hat_openstack_platform/8/html/director_installation_and_usage/appe-bonding_options

	// BondMode specifies the bond mode.
	// [balance-slb / balance-tcp / active-backup]
	BondMode string

	// LACP specifies the LACP mode.
	// [active / passive / off]
	LACP string

	// LACPTime specifies the LACP heartbeat to 1 second (fast) or 30 seconds (slow)
	// [fast / slow]
	LACPTime string

	// LACPFallbackAB specifies the LACP behavior to switch to bond_mode=active-backup as a fallback.
	// [true / false]
	LACPFallbackAB string

	// Members specifies the members interfaces of the bond.
	Members map[string]InterfaceOptions

	// Options specifies additional a to be set on the bond.
	// format: ["option1=value1", "option2=value2", ...]
	Options []string
}

// slice creates a string slice containing any non-zero option values from the
// struct in the format expected by Open vSwitch.
func (o BondOptions) slice() []string {
	var s []string
	// 1. option field must between <InterfaceName> and <Set Interface>
	// 2. members can be out-of-order, but in-order can improve readability
	// example: ovs-vsctl --no-wait add-bond br0-ovs dpdkbond0
	// dpdk0 dpdk1
	// lacp=active
	// -- set Interface dpdk0 type=dpdk options:dpdk-devargs=0000:00:10.0
	// -- set Interface dpdk1 type=dpdk options:dpdk-devargs=0000:00:10.1

	keys := slices.Sorted(maps.Keys(o.Members))
	s = append(s, keys...)

	if o.BondMode != "" {
		s = append(s, fmt.Sprintf("bond_mode=%s", o.BondMode))
	}

	if o.LACP != "" {
		s = append(s, fmt.Sprintf("lacp=%s", o.LACP))
	}

	if o.LACPTime != "" {
		s = append(s, fmt.Sprintf("other_config:lacp-time=%s", o.LACPTime))
	}

	if o.LACPFallbackAB != "" {
		s = append(s, fmt.Sprintf("other_config:lacp-fallback-ab=%s", o.LACPFallbackAB))
	}

	for _, name := range keys {
		opt := o.Members[name].slice()
		s = append(s, fmt.Sprintf("-- set Interface %s %s", name, strings.Join(opt, " ")))
	}

	if len(o.Options) > 0 {
		s = append(s, fmt.Sprintf("options=%s", strings.Join(o.Options, ",")))
	}

	return s
}

// Interface sets configuration for an interface using the values from an
// InterfaceOptions struct.
func (v *VSwitchSetService) Interface(ifi string, options InterfaceOptions) error {
	// Prepend command line arguments before expanding options slice
	// and appending it
	args := []string{"set", "interface", ifi}
	args = append(args, options.slice()...)

	_, err := v.v.exec(args...)
	return err
}

// An InterfaceOptions struct enables configuration of an Interface.
type InterfaceOptions struct {
	// Type specifies the Open vSwitch interface type.
	Type InterfaceType

	// Peer specifies an interface to peer with when creating a patch interface.
	Peer string

	// MTURequest specifies the maximum transmission unit associated with an
	// interface.
	MTURequest int

	// Ingress Policing
	//
	// These settings control ingress policing for packets received on this
	// interface.  On a physical interface, this limits the rate at which
	// traffic is allowed into the system from the outside; on a virtual
	// interface (one connected to a virtual machine), this limits the rate
	// at which the VM is able to transmit.

	// IngressRatePolicing specifies the maximum rate for data received on
	// this interface in kbps.  Data received faster than this rate is dropped.
	// Set to 0 (the default) to disable policing.
	IngressRatePolicing int64

	// IngressBurstPolicing specifies the maximum burst size for data received on
	// this interface in kb.  The default burst size if set to 0 is 1000 kb.
	// This value has no effect if IngressRatePolicing is set to 0.  Specifying
	// a larger burst size lets the algorithm be more forgiving, which is important
	// for protocols like TCP that react severely to dropped packets.  The burst
	// size should be at least the size of the interface's MTU.  Specifying a
	// value that is numerically at least as large as 10% of IngressRatePolicing
	// helps TCP come closer to achieving the full rate.
	IngressBurstPolicing int64

	// RemoteIP can be populated when the interface is a tunnel interface type
	// for example "stt" or "vxlan". It specifies the remote IP address with which to
	// form tunnels when traffic is sent to this port. Optionally it could be set to
	// "flow" which expects the flow to set tunnel destination.
	RemoteIP string

	// Key can be populated when the interface is a tunnel interface type
	// for example "stt" or "vxlan". It specifies the tunnel ID to attach to
	// tunneled traffic leaving this interface. Optionally it could be set to
	// "flow" which expects the flow to set tunnel ID.
	Key string

	// DPDKDevArgs can be populated when the interface is a dpdk interface type.
	// It specifies the DPDK device arguments for the interface.
	// format: "dpdk-devargs=0000:01:00.0"
	DPDKDevArgs string

	// VhostServerPath can be populated when the interface is a dpdk vhostuserclient interface type.
	// It specifies the vhost-server-path option for the interface.
	// format: "vhost-server-path=/var/run/vhostuserclient/vhost-user-client-1"
	VhostServerPath string

	// Options specifies additional a to be set on the interface.
	// format: ["option1=value1", "option2=value2", ...]
	Options []string
}

// slice creates a string slice containing any non-zero option values from the
// struct in the format expected by Open vSwitch.
func (i InterfaceOptions) slice() []string {
	var s []string

	if i.Type != "" {
		s = append(s, fmt.Sprintf("type=%s", i.Type))
	}

	if i.Peer != "" {
		s = append(s, fmt.Sprintf("options:peer=%s", i.Peer))
	}

	if i.MTURequest > 0 {
		s = append(s, fmt.Sprintf("mtu_request=%d", i.MTURequest))
	}

	if i.IngressRatePolicing == DefaultIngressRatePolicing {
		// Set to 0 (the default) to disable policing.
		s = append(s, "ingress_policing_rate=0")
	} else if i.IngressRatePolicing > 0 {
		s = append(s, fmt.Sprintf("ingress_policing_rate=%d", i.IngressRatePolicing))
	}

	if i.IngressBurstPolicing == DefaultIngressBurstPolicing {
		// Set to 0 (the default) to the default burst size.
		s = append(s, "ingress_policing_burst=0")
	} else if i.IngressBurstPolicing > 0 {
		s = append(s, fmt.Sprintf("ingress_policing_burst=%d", i.IngressBurstPolicing))
	}

	if i.RemoteIP != "" {
		s = append(s, fmt.Sprintf("options:remote_ip=%s", i.RemoteIP))
	}

	if i.Key != "" {
		s = append(s, fmt.Sprintf("options:key=%s", i.Key))
	}

	if i.DPDKDevArgs != "" {
		s = append(s, fmt.Sprintf("options:dpdk-devargs=%s", i.DPDKDevArgs))
	}

	if i.VhostServerPath != "" {
		s = append(s, fmt.Sprintf("options:vhost-server-path=%s", i.VhostServerPath))
	}

	if len(i.Options) > 0 {
		s = append(s, fmt.Sprintf("options=%s", strings.Join(i.Options, ",")))
	}

	return s
}
