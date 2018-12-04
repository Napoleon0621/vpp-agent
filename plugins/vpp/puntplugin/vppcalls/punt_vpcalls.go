// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vppcalls

import (
	api_punt "github.com/ligato/vpp-agent/plugins/vpp/binapi/punt"
	"github.com/ligato/vpp-agent/plugins/vpp/model/punt"
)

// RegisterPuntSocket registers new punt to socket
func (h *PuntVppHandler) RegisterPuntSocket(puntCfg *punt.Punt) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.registerPuntWithSocketIPv4(puntCfg)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.registerPuntWithSocketIPv6(puntCfg)
	}
	// if L3 is set to all, register both, IPv4 and IPv6
	err := h.registerPuntWithSocketIPv4(puntCfg)
	if err != nil {
		return err
	}
	return h.registerPuntWithSocketIPv6(puntCfg)
}

// DeregisterPuntSocket removes existing punt to socket registration
func (h *PuntVppHandler) DeregisterPuntSocket(puntCfg *punt.Punt) error {
	if puntCfg.L3Protocol == punt.L3Protocol_IPv4 {
		return h.registerPuntWithSocketIPv4(puntCfg)
	} else if puntCfg.L3Protocol == punt.L3Protocol_IPv6 {
		return h.registerPuntWithSocketIPv6(puntCfg)
	}
	// if L3 is set to all, deregister both, IPv4 and IPv6
	err := h.registerPuntWithSocketIPv4(puntCfg)
	if err != nil {
		return err
	}
	return h.registerPuntWithSocketIPv6(puntCfg)
}

func (h *PuntVppHandler) registerPuntWithSocketIPv4(punt *punt.Punt) error {
	return h.registerPuntWithSocket(punt, true)
}

func (h *PuntVppHandler) registerPuntWithSocketIPv6(punt *punt.Punt) error {
	return h.registerPuntWithSocket(punt, false)
}

func (h *PuntVppHandler) registerPuntWithSocket(punt *punt.Punt, isIPv4 bool) error {
	pathName := []byte(punt.SocketPath)
	pathByte := make([]byte, 108) // linux sun_path defined to 108 bytes as by unix(7)
	for i, c := range pathName {
		pathByte[i] = c
	}

	req := &api_punt.PuntSocketRegister{
		HeaderVersion: 1,
		IsIP4:         boolToUint(isIPv4),
		L4Protocol:    resolveL4Proto(punt.L4Protocol),
		L4Port:        uint16(punt.Port),
		Pathname:      pathByte,
	}
	reply := &api_punt.PuntSocketRegisterReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func (h *PuntVppHandler) unregisterPuntWithSocketIPv4(punt *punt.Punt) error {
	return h.unregisterPuntWithSocket(punt, true)
}

func (h *PuntVppHandler) unregisterPuntWithSocketIPv6(punt *punt.Punt) error {
	return h.unregisterPuntWithSocket(punt, false)
}

func (h *PuntVppHandler) unregisterPuntWithSocket(punt *punt.Punt, isIPv4 bool) error {
	req := &api_punt.PuntSocketDeregister{
		IsIP4:      boolToUint(isIPv4),
		L4Protocol: resolveL4Proto(punt.L4Protocol),
		L4Port:     uint16(punt.Port),
	}
	reply := &api_punt.PuntSocketDeregisterReply{}

	if err := h.callsChannel.SendRequest(req).ReceiveReply(reply); err != nil {
		return err
	}

	return nil
}

func resolveL4Proto(protocol punt.L4Protocol) uint8 {
	switch protocol {
	case punt.L4Protocol_TCP:
		return uint8(punt.L4Protocol_TCP)
	case punt.L4Protocol_UDP:
		return uint8(punt.L4Protocol_UDP)
	}
	return uint8(punt.L4Protocol_UNDEFINED_L4)
}

func boolToUint(input bool) uint8 {
	if input {
		return 1
	}
	return 0
}