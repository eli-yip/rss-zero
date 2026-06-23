package tombkeeper

// tombkeeperAccounts holds tombkeeper's weibo accounts. The set is fixed (two
// accounts that both appear on tombkeeper.io), so it is hardcoded here rather
// than stored in the database. This can later be abstracted behind an interface
// if the accounts ever need to be configurable.
var tombkeeperAccounts = map[string]string{
	"1401527553": "tombkeeper",
	"6827625527": "t0mbkeeper",
}

// IsTombkeeperUID reports whether uid belongs to one of tombkeeper's accounts.
// It is used both for classifying crawled posts and for deciding whether a
// "微博正文" link points at tombkeeper's own weibo.
func IsTombkeeperUID(uid string) bool {
	_, ok := tombkeeperAccounts[uid]
	return ok
}
