// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package node

import (
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/params/vars"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	DefaultHTTPHost = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 8545        // Default TCP port for the HTTP RPC server
	DefaultWSHost   = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort   = 8546        // Default TCP port for the websocket RPC server
	DefaultAuthHost = "localhost" // Default host interface for the authenticated apis
	DefaultAuthPort = 8551        // Default port for the authenticated apis
)

const (
	// Engine API batch limits: these are not configurable by users, and should cover the
	// needs of all CLs.
	engineAPIBatchItemLimit         = 2000
	engineAPIBatchResponseSizeLimit = 250 * 1000 * 1000
	engineAPIBodyLimit              = 128 * 1024 * 1024
)

var (
	DefaultAuthCors    = []string{"localhost"} // Default cors domain for the authenticated apis
	DefaultAuthVhosts  = []string{"localhost"} // Default virtual hosts for the authenticated apis
	DefaultAuthOrigins = []string{"localhost"} // Default origins for the authenticated apis
	DefaultAuthPrefix  = ""                    // Default prefix for the authenticated apis
	DefaultAuthModules = []string{"eth", "engine"}
)

// EthernovaDefaultHTTPModules is the API namespace whitelist exposed via HTTP
// RPC by default when no --http.api flag is provided. It is intentionally
// broader than upstream go-ethereum so an Ethernova node started with no flags
// at all is immediately usable by tooling (Hardhat, ethers, the nova-sdk and
// the Phase 8 test suite), without being unsafe to expose:
//
//   - eth, net, web3, txpool: the standard EVM client surface.
//   - miner:                  miner control (PoW devnet defaults).
//   - nova, ethernova:        Phase 8 namespaces required by the nova SDK and
//                             the Hardhat plugin. Both names share the same
//                             service in eth/backend.go.
//
// Deliberately NOT in the default list:
//
//   - admin:    peer/datadir manipulation. Add with --http.api admin if needed.
//   - personal: account creation/unlock. Deprecated upstream; keep opt-in.
//   - debug:    expensive trace_* and dump methods. Add with --http.api debug.
//
// To expose the node externally (LAN, public IP) you still need to override
// the bind address explicitly with --http.addr 0.0.0.0 -- the default below
// keeps HTTP loopback-only (localhost) so a bare invocation is production-safe.
var EthernovaDefaultHTTPModules = []string{
	"eth", "net", "web3", "txpool", "miner", "nova", "ethernova",
}

// EthernovaDefaultWSModules mirrors EthernovaDefaultHTTPModules for the
// websocket transport. Tooling that prefers WS (some hardhat plugins, real-time
// log subscribers) should work out of the box on ws://localhost:8546.
var EthernovaDefaultWSModules = []string{
	"eth", "net", "web3", "txpool", "miner", "nova", "ethernova",
}

// DefaultConfig contains reasonable default settings.
//
// Ethernova customization: HTTPHost and WSHost default to loopback so that an
// `ethernova.exe` invocation with no flags exposes both RPC transports on
// localhost. Upstream go-ethereum leaves these empty, which disables both
// servers by default; that is too unfriendly for a single-binary devnet/PoW
// chain where the operator is expected to talk to the node from tools on the
// same machine. External exposure still requires an explicit --http.addr /
// --ws.addr override -- see EthernovaDefaultHTTPModules comment above.
var DefaultConfig = Config{
	DataDir:              vars.DefaultDataDir(),
	HTTPHost:             DefaultHTTPHost,
	HTTPPort:             DefaultHTTPPort,
	AuthAddr:             DefaultAuthHost,
	AuthPort:             DefaultAuthPort,
	AuthVirtualHosts:     DefaultAuthVhosts,
	HTTPModules:          EthernovaDefaultHTTPModules,
	HTTPVirtualHosts:     []string{"localhost"},
	HTTPTimeouts:         rpc.DefaultHTTPTimeouts,
	WSHost:               DefaultWSHost,
	WSPort:               DefaultWSPort,
	WSModules:            EthernovaDefaultWSModules,
	BatchRequestLimit:    1000,
	BatchResponseMaxSize: 25 * 1000 * 1000,
	GraphQLVirtualHosts:  []string{"localhost"},
	P2P: p2p.Config{
		ListenAddr: ":30303",
		MaxPeers:   50,
		NAT:        nat.Any(),
	},
	DBEngine: "", // Use whatever exists, will default to Pebble if non-existent and supported
}