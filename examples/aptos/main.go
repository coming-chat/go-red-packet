package main

import (
	"os"

	"github.com/coming-chat/go-red-packet/redpacket"
	"github.com/coming-chat/wallet-SDK/core/aptos"
)

const (
	testNetUrl = "https://fullnode.devnet.aptoslabs.com"
	faucetUrl  = "https://faucet.devnet.aptoslabs.com"
)

func main() {
	chain := aptos.NewChainWithRestUrl(testNetUrl)
	account, err := aptos.AccountWithPrivateKey(os.Getenv("private"))
	if err != nil {
		panic(err)
	}
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeAptos, chain, os.Getenv("red_packet"))
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
	txDetail, err := contract.FetchRedPacketCreationDetail(txHash)
	if err != nil {
		panic(err)
	}
	println(txHash)
	println(txDetail.Status)
}
