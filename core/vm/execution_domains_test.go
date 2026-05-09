package vm

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestInspectExecutionDomain(t *testing.T) {
	tests := []struct {
		name      string
		code      []byte
		domain    ExecutionDomain
		prefixLen int
		runtime   []byte
	}{
		{name: "legacy empty", code: nil, domain: DomainLegacy, prefixLen: 0, runtime: nil},
		{name: "legacy bytecode", code: []byte{0x60, 0x00}, domain: DomainLegacy, prefixLen: 0, runtime: []byte{0x60, 0x00}},
		{name: "domain 1", code: []byte{0xEF, 0x01, 0x60, 0x01}, domain: DomainNova, prefixLen: 2, runtime: []byte{0x60, 0x01}},
		{name: "domain 2", code: []byte{0xEF, 0x02, 0x60, 0x02}, domain: DomainChannel, prefixLen: 2, runtime: []byte{0x60, 0x02}},
		{name: "unknown ef prefix", code: []byte{0xEF, 0x03, 0x60, 0x03}, domain: DomainLegacy, prefixLen: 0, runtime: []byte{0xEF, 0x03, 0x60, 0x03}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, prefixLen := InspectExecutionDomain(tt.code)
			if domain != tt.domain || prefixLen != tt.prefixLen {
				t.Fatalf("InspectExecutionDomain() = (%d,%d), want (%d,%d)", domain, prefixLen, tt.domain, tt.prefixLen)
			}
			parsedDomain, runtime := ParseExecutionDomain(tt.code)
			if parsedDomain != tt.domain {
				t.Fatalf("ParseExecutionDomain domain = %d, want %d", parsedDomain, tt.domain)
			}
			if !bytes.Equal(runtime, tt.runtime) {
				t.Fatalf("runtime = %x, want %x", runtime, tt.runtime)
			}
		})
	}
}

func TestCapabilityHelpers(t *testing.T) {
	if DefaultCapabilitiesForDomain(DomainLegacy) != CapabilityNone {
		t.Fatalf("legacy domain should have no contract capabilities")
	}
	if DefaultCapabilitiesForDomain(DomainNova)&CapabilitySessionArbiter == 0 {
		t.Fatalf("domain 1 should include session arbiter capability")
	}
	addr := common.HexToAddress("0x2d")
	if RequiredCapabilityForPrecompile(addr) != CapabilitySessionArbiter {
		t.Fatalf("0x2D should require session arbiter capability")
	}
	names := CapabilityNames(CapabilityProtocolObjects | CapabilityMailboxOps)
	if len(names) != 2 || names[0] != "protocolObjects" || names[1] != "mailboxOps" {
		t.Fatalf("unexpected capability names: %#v", names)
	}
}
