# go-red-packet

- [go-red-packet](#go-red-packet)
	- [创建红包](#创建红包)
	- [红包费用](#红包费用)
	- [FetchRedPacketCreationDetail 的 error 返回](#fetchredpacketcreationdetail-的-error-返回)

A client for red packet contract.

此项目只提供与 redpacket 合约交互的交易与方法封装，不提供发送交易。项目使用 [coming-chat/wallet-sdk](https://github.com/coming-chat/wallet-SDK) 实现发送交易。


## 创建红包

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
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeAptos, chain, os.Getenv("red_packet"), nil)
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
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeEth, chain, os.Getenv("red_packet"), nil)
	if err != nil {
		panic(err)
	}
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

sui 创建红包实例
```go
redpacketPackageId := "0xa18d087873b719be07b3e24506ec260d1fed88d7"
contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeSui, chain, redpacketPackageId, &redpacket.ContractConfig{
	SuiConfigAddress: "0xba6780a9a4701cb00b979a3393a189e8adf51022",
})
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
```

## 红包费用

发红包的费用分为两部分
1. gas fee
2. 合约服务费
   1. eth 红包合约收取的是链原声币作为服务费
   2. aptos 红包合约收取的是当前代币作为服务费（目前仅支持原生币红包）
   3. sui 红包合约收取的是当前呆逼作为服务费

`RedPacketContract` 接口方法 `EstimateFee(*RedPacketAction) (string, error)` 获取合约服务费。
方法 `EstimateGasFee(base.Account, *RedPacketAction) (string, error)` 获取 gas fee （gasLimit * gasPrice）。

## FetchRedPacketCreationDetail 的 error 返回

error 分为两类，一类是红包数据错误（包括 hash 对应的交易不存在）；一类是其他错误（网络错误等）
```go
_, err = contract.FetchRedPacketCreationDetail("0x1908acf431fde3cc31926860c342f18421669d087325defa19cfe42537738c21")
switch e := err.(type) {
case *redpacket.RedPacketDataError:
	println(e.Error())
default:
	println("other error")
}
```
