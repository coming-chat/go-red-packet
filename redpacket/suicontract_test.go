package redpacket

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/coming-chat/go-sui/v2/types"
	"github.com/coming-chat/wallet-SDK/core/sui"
	"github.com/stretchr/testify/require"
)

var Mnemonic = os.Getenv("WalletSdkTestM3")

const (
	MainnetRpcUrl            = "https://sui-mainnet.coming.chat"
	redPacketContractAddress = "0xf5244fdbeae35291fd829d5dd13cf8ce596c986ca1373687600808ee6d7c0241"
	configAddress            = "0x1029909aa0c52524de0ce602cc80b52a17c3962b07fac67f982e6388c72be2e7"

	SuiCoinType = "0x2::sui::SUI"
)

func MainnetChain() *sui.Chain {
	return sui.NewChainWithRpcUrl(MainnetRpcUrl)
}

func EnvAccount(t *testing.T) *sui.Account {
	acc, err := sui.NewAccountWithMnemonic(Mnemonic)
	require.Nil(t, err)
	return acc
}

func TestSui_RedPacketCreate(t *testing.T) {
	account := EnvAccount(t)
	chain := MainnetChain()
	config := ContractConfig{
		SuiConfigAddress: configAddress,
	}
	contract, err := NewSuiRedPacketContract(chain, redPacketContractAddress, &config)
	require.Nil(t, err)
	suiContract, ok := contract.(*suiRedPacketContract)
	require.True(t, ok)

	createAction, err := NewRedPacketActionCreate(SuiCoinType, 1, "10000000")
	require.Nil(t, err)

	txn, err := suiContract.createTx(account, createAction)
	require.Nil(t, err)

	simulateCheck(t, chain, txn, true)
}

func simulateCheck(t *testing.T, chain *sui.Chain, txn *sui.Transaction, showJson bool) *types.DryRunTransactionBlockResponse {
	cli, err := chain.Client()
	require.Nil(t, err)
	resp, err := cli.DryRunTransaction(context.Background(), txn.TransactionBytes())
	require.Nil(t, err)
	require.Equal(t, resp.Effects.Data.V1.Status.Error, "")
	require.True(t, resp.Effects.Data.IsSuccess())
	if showJson {
		data, err := json.Marshal(resp)
		require.Nil(t, err)
		respStr := string(data)
		t.Log("simulate run resp: ", respStr)
	}
	t.Log("simulate gas price = ", resp.Effects.Data.GasFee())
	return resp
}
