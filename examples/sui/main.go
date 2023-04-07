package main

import (
	"os"

	"github.com/coming-chat/go-red-packet/redpacket"
	"github.com/coming-chat/wallet-SDK/core/sui"
)

const (
	tokenAddress = "0x2::sui::SUI"
)

func main() {
	rpcUrl := os.Getenv("rpc")
	chain := sui.NewChainWithRpcUrl(rpcUrl)
	account, err := sui.AccountWithPrivateKey(os.Getenv("private"))
	if err != nil {
		panic(err)
	}
	println(account.Address())
	redpacketPackageId := "0xb0bf0468d3f0225d0b2011f1c485daac2c4543b11f35023aae7d3a1f64a3c2c6"
	contract, err := redpacket.NewRedPacketContract(redpacket.ChainTypeSui, chain, redpacketPackageId, &redpacket.ContractConfig{
		SuiConfigAddress: "0x2a23d0d7f993609d8e8c0af87d39635f9ed03cb2d19065464c5c57052ebc751b",
	})
	if err != nil {
		panic(err)
	}
	action, err := redpacket.NewRedPacketActionCreate(tokenAddress, 5, "100000")
	if err != nil {
		panic(err)
	}
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
	// action, err := redpacket.NewSuiRedPacketActionClose(tokenAddress, "0x58f22d673e21a90d99511ffbb28c854c415c3255", "", "")
	// if err != nil {
	// 	panic(err)
	// }
	gasFee, err := contract.EstimateGasFee(account, action)
	if err != nil {
		panic(err)
	}
	println(gasFee)
	txHash, err := contract.SendTransaction(account, action)
	if err != nil {
		panic(err)
	}
	// txHash := "GCZsXn8qpDsuadYbFGiunM45CWzZZ3YoM6tRjne9b37s"
	println(txHash)
	txDetail, err := contract.FetchRedPacketCreationDetail(txHash)
	if err != nil {
		panic(err)
	}
	println(txDetail.Status)
}
