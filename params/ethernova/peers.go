package ethernova

// DefaultPublicPeers are enode URLs of public Ethernova devnet nodes that every
// build dials automatically so a user running geth with no flags (double-click
// on Windows) still joins the network without needing admin.addPeer.
var DefaultPublicPeers = []string{
	"enode://ef42544ea59225e0d9b37482cb999302b064908af9c904801513d5e20913c0f03d3ed6318126773921c74c97d84c8c6e19df93d0016fb838ab66da2215e3abc0@207.180.230.125:30301",
}
