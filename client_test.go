// Copyright 2014 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"net/url"
	"testing"
)

var hostPortNoPortTests = []struct {
	u                    *url.URL
	hostPort, hostNoPort string
}{
	{&url.URL{Scheme: "ws", Host: "example.com"}, "example.com:80", "example.com"},
	{&url.URL{Scheme: "wss", Host: "example.com"}, "example.com:443", "example.com"},
	{&url.URL{Scheme: "ws", Host: "example.com:7777"}, "example.com:7777", "example.com"},
	{&url.URL{Scheme: "wss", Host: "example.com:7777"}, "example.com:7777", "example.com"},
	{&url.URL{Scheme: "ws", Host: "[::1]"}, "[::1]:80", "::1"},
	{&url.URL{Scheme: "wss", Host: "[::1]"}, "[::1]:443", "::1"},
	{&url.URL{Scheme: "ws", Host: "[::1]:8080"}, "[::1]:8080", "::1"},
	{&url.URL{Scheme: "wss", Host: "[::1]:8080"}, "[::1]:8080", "::1"},
	{&url.URL{Scheme: "ws", Host: "::1"}, "[::1]:80", "::1"},
	{&url.URL{Scheme: "wss", Host: "::1"}, "[::1]:443", "::1"},
	{&url.URL{Scheme: "ws", Host: "2001:67c:ae8:3::3b"}, "[2001:67c:ae8:3::3b]:80", "2001:67c:ae8:3::3b"},
	{&url.URL{Scheme: "wss", Host: "2001:67c:ae8:3::3b"}, "[2001:67c:ae8:3::3b]:443", "2001:67c:ae8:3::3b"},
	{&url.URL{Scheme: "ws", Host: "[2001:67c:ae8:3::3b]"}, "[2001:67c:ae8:3::3b]:80", "2001:67c:ae8:3::3b"},
	{&url.URL{Scheme: "ws", Host: "[2001:67c:ae8:3::3b]:8080"}, "[2001:67c:ae8:3::3b]:8080", "2001:67c:ae8:3::3b"},
	{&url.URL{Scheme: "ws", Host: "[fe80::1%eth0]"}, "[fe80::1%eth0]:80", "fe80::1%eth0"},
	{&url.URL{Scheme: "ws", Host: "[fe80::1%eth0]:8080"}, "[fe80::1%eth0]:8080", "fe80::1%eth0"},
	{&url.URL{Scheme: "ws", Host: "fe80::1%eth0"}, "[fe80::1%eth0]:80", "fe80::1%eth0"},
}

func TestHostPortNoPort(t *testing.T) {
	for _, tt := range hostPortNoPortTests {
		hostPort, hostNoPort := hostPortNoPort(tt.u)
		if hostPort != tt.hostPort {
			t.Errorf("hostPortNoPort(%v) returned hostPort %q, want %q", tt.u, hostPort, tt.hostPort)
		}
		if hostNoPort != tt.hostNoPort {
			t.Errorf("hostPortNoPort(%v) returned hostNoPort %q, want %q", tt.u, hostNoPort, tt.hostNoPort)
		}
	}
}
