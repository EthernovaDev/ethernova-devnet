package ethernova

// DefaultPublicPeers are enode URLs of public Ethernova devnet nodes that every
// build dials automatically so a user running geth with no flags (double-click
// on Windows) still joins the network without needing admin.addPeer.
var DefaultPublicPeers = []string{
	"enode://c56f025f5df73df9e9415f9df459dc7fbe204875e5972fda9cdc61ef4b4f7164aa2f886120f6352665eabedbfeda96119cd7c0f76e637f0752cff0783fa982d8@207.180.230.125:30301",
}
