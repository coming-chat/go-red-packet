package main

import (
	"os"

	"github.com/coming-chat/go-red-packet/redpacket"
	"github.com/coming-chat/wallet-SDK/core/sui"
)

const (
	// devNetUrl = "https://fullnode.devnet.sui.io"
	devNetUrl    = "https://wallet-rpc.devnet.sui.io/"
	tokenAddress = "0x2::sui::SUI"
)

func main() {
	chain := sui.NewChainWithRpcUrl(devNetUrl)
	account, err := sui.AccountWithPrivateKey(os.Getenv("private"))
	if err != nil {
		panic(err)
	}
	redpacketPackageId := "0xa18d087873b719be07b3e24506ec260d1fed88d7"
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeSui, chain, redpacketPackageId, &redpacket.ContractConfig{
		SuiConfigAddress: "0xba6780a9a4701cb00b979a3393a189e8adf51022",
	})
	if err != nil {
		panic(err)
	}
	// action, err := redpacket.NewRedPacketActionCreate(tokenAddress, 5, "100000")
	// if err != nil {
	// 	panic(err)
	// }
	// action, err := redpacket.NewSuiRedpacketActionOpen(tokenAddress, "0x58f22d673e21a90d99511ffbb28c854c415c3255", []string{
	// 	account.Address(),
	// 	account.Address(),
	// 	account.Address(),
	// 	account.Address(),
	// 	account.Address(),
	// }, []string{
	// 	"20000",
	// 	"20000",
	// 	"20000",
	// 	"20000",
	// 	"17500",
	// })
	action, err := redpacket.NewSuiRedPacketActionClose(tokenAddress, "0x58f22d673e21a90d99511ffbb28c854c415c3255", "", "")
	if err != nil {
		panic(err)
	}
	gasFee, err := contract.EstimateGasFee(account, action)
	if err != nil {
		panic(err)
	}
	println(gasFee)
	txHash, err := contract.SendTransaction(account, action)
	if err != nil {
		panic(err)
	}
	// txHash := "EXEVapW35HnhtyJ7htwhMwW9a3KQsnn7h6oZ97SXahRD"
	txDetail, err := contract.FetchRedPacketCreationDetail(txHash)
	if err != nil {
		panic(err)
	}
	println(txHash)
	println(txDetail.Status)
}
