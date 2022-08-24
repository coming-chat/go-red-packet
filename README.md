# go-red-packet

A client for red packet contract.

此项目只提供与 redpacket 合约交互的交易与方法封装，不提供发送交易。项目使用 [coming-chat/wallet-sdk](https://github.com/coming-chat/wallet-SDK) 实现发送交易。

aptos 创建红包示例：
```go
func main() {
    // 使用 coming-chat/wallet-SDK 创建 aptos 链的 chain/account
	chain := aptos.NewChainWithRestUrl(testNetUrl)
	account, err := aptos.AccountWithPrivateKey(os.Getenv("private"))
	if err != nil {
		panic(err)
	}
	
	// 创建合约对象，以及想要执行的 action
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeAptos, chain, os.Getenv("red_packet"))
	if err != nil {
		panic(err)
	}
	action, err := redpacket.NewRedPacketActionCreate("", 5, "100000")
	if err != nil {
		panic(err)
	}

	// 使用合约对象发送 action 交易到链上
	txHash, err := contract.SendTransaction(account, action)
	if err != nil {
		panic(err)
	}
	txDetail, err := chain.FetchTransactionDetail(txHash)
	if err != nil {
		panic(err)
	}
	println(txHash)
	println(txDetail.Status)
}

```

eth 创建红包示例：
```go
func main() {
	chain := eth.NewChainWithRpc(os.Getenv("rpc"))
	account, err := eth.NewAccountWithMnemonic(os.Getenv(""))
	if err != nil {
		panic(err)
	}
	contract := redpacket.NewEthRedPacketContract(chain, os.Getenv("red_packet"))
	action, err := redpacket.NewRedPacketActionCreate("", 5, "100000")
	if err != nil {
		panic(err)
	}
	txHash, err := contract.SendTransaction(account, action)
	if err != nil {
		panic(err)
	}
	txDetail, err := chain.FetchTransactionDetail(txHash)
	if err != nil {
		panic(err)
	}
	println(txHash)
	println(txDetail.Status)
}
```