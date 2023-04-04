package redpacket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/coming-chat/go-sui/client"
	"github.com/coming-chat/go-sui/types"
	"github.com/coming-chat/wallet-SDK/core/base"
	"github.com/coming-chat/wallet-SDK/core/sui"
)

const (
	SuiDecimal   = 9
	suiGasBudget = 10000

	suiPackage     = "red_packet"
	suiCoinAddress = "0x2::sui::SUI"
)

type suiRedPacketContract struct {
	chain        sui.IChain
	address      string
	packageIdHex types.HexData
	configHex    types.HexData
}

func NewSuiRedPacketContract(chain sui.IChain, contractAddress string, config *ContractConfig) (RedPacketContract, error) {
	address := "0x" + strings.TrimPrefix(contractAddress, "0x")
	configHex, err := types.NewHexData(config.SuiConfigAddress)
	if err != nil {
		return nil, err
	}
	pkgId, err := types.NewHexData(address)
	if err != nil {
		return nil, err
	}
	return &suiRedPacketContract{
		chain:        chain,
		address:      address,
		packageIdHex: *pkgId,
		configHex:    *configHex,
	}, nil
}

func (c *suiRedPacketContract) SendTransaction(account base.Account, rpa *RedPacketAction) (string, error) {
	tx, err := c.createTx(account, rpa)
	if err != nil {
		return "", err
	}

	suiAccount, ok := account.(*sui.Account)
	if !ok {
		return "", errors.New("invalid account object")
	}

	signedTxn, err := tx.SignWithAccount(suiAccount)
	if err != nil {
		return "", err
	}
	bytes, err := json.Marshal(signedTxn)
	if err != nil {
		return "", err
	}
	txnString := types.Bytes(bytes).GetBase64Data().String()
	return c.chain.SendRawTransaction(txnString)
}

func (c *suiRedPacketContract) createTx(account base.Account, rpa *RedPacketAction) (*sui.Transaction, error) {
	cli, err := c.chain.GetClient()
	if err != nil {
		return nil, err
	}
	addr, err := types.NewAddressFromHex(account.Address())
	if err != nil {
		return nil, err
	}
	switch rpa.Method {
	case RPAMethodCreate:
		coins, gas, err := c.pickCoinsAndGas(cli, account, rpa)
		if err != nil {
			return nil, err
		}
		args := []interface{}{
			c.configHex,
			coins,
			strconv.Itoa(rpa.CreateParams.Count),
			rpa.CreateParams.Amount,
		}
		tx, err := cli.MoveCall(
			context.Background(),
			*addr,
			c.packageIdHex,
			suiPackage,
			"create",
			[]string{rpa.CreateParams.TokenAddress},
			args,
			gas,
			suiGasBudget,
		)
		if err != nil {
			return nil, err
		}
		return &sui.Transaction{
			Txn:          *tx,
			MaxGasBudget: suiGasBudget,
		}, nil
	case RPAMethodOpen:
		if len(rpa.OpenParams.PacketObjectId) == 0 {
			return nil, errors.New("invalid redPacketObjectId")
		}
		redPacketObjectId, err := types.NewHexData(rpa.OpenParams.PacketObjectId)
		if err != nil {
			return nil, err
		}
		addresses := make([]*types.HexData, len(rpa.OpenParams.Addresses))
		for i := range rpa.OpenParams.Addresses {
			addresses[i], err = types.NewHexData(rpa.OpenParams.Addresses[i])
			if err != nil {
				return nil, err
			}
		}
		args := []interface{}{
			redPacketObjectId,
			addresses,
			rpa.OpenParams.Amounts,
		}
		gas, err := c.pickGas(cli, account, suiGasBudget)
		if err != nil {
			return nil, err
		}
		tx, err := cli.MoveCall(
			context.Background(),
			*addr,
			c.packageIdHex,
			suiPackage,
			"open",
			[]string{rpa.OpenParams.TokenAddress},
			args,
			gas,
			suiGasBudget,
		)
		if err != nil {
			return nil, err
		}
		return &sui.Transaction{
			Txn:          *tx,
			MaxGasBudget: suiGasBudget,
		}, nil
	case RPAMethodClose:
		if len(rpa.CloseParams.PacketObjectId) == 0 {
			return nil, errors.New("invalid redPacketObjectId")
		}
		packetId, err := types.NewHexData(rpa.CloseParams.PacketObjectId)
		if err != nil {
			return nil, err
		}
		gas, err := c.pickGas(cli, account, suiGasBudget)
		if err != nil {
			return nil, err
		}
		args := []interface{}{
			packetId,
		}
		tx, err := cli.MoveCall(
			context.Background(),
			*addr,
			c.packageIdHex,
			suiPackage,
			"close",
			[]string{rpa.CloseParams.TokenAddress},
			args,
			gas,
			suiGasBudget,
		)
		if err != nil {
			return nil, err
		}
		return &sui.Transaction{
			Txn:          *tx,
			MaxGasBudget: suiGasBudget,
		}, nil
	default:
		return nil, fmt.Errorf("unsopported red packet method %s", rpa.Method)
	}
}

func (c *suiRedPacketContract) pickGas(cli *client.Client, account base.Account, gasBudget uint64) (*types.ObjectId, error) {
	ctx := context.Background()
	addressHex, _ := types.NewHexData(account.Address())
	suiCoins, err := cli.GetSuiCoinsOwnedByAddress(ctx, *addressHex)
	if err != nil {
		return nil, err
	}
	gasCoin, err := suiCoins.PickCoinNoLess(gasBudget)
	if err != nil {
		return nil, err
	}
	return &gasCoin.Reference().ObjectId, nil
}

func (c *suiRedPacketContract) pickCoinsAndGas(cli *client.Client, account base.Account, rpa *RedPacketAction) ([]types.ObjectId, *types.ObjectId, error) {
	ctx := context.Background()
	addressHex, _ := types.NewHexData(account.Address())
	allCoinsStruct, err := cli.GetCoins(ctx, *addressHex, &rpa.CreateParams.TokenAddress, nil, 100)
	allCoins := make(types.Coins, 0)
	for _, coin := range allCoinsStruct.Data {
		allCoins = append(allCoins, coin)
	}
	if err != nil {
		return nil, nil, err
	}

	amountBig, b := big.NewInt(0).SetString(rpa.CreateParams.Amount, 10)
	var (
		coins   types.Coins
		gasCoin *types.Coin
	)
	if !b {
		return nil, nil, errors.New("invalid amount")
	}
	if rpa.CreateParams.TokenAddress == suiCoinAddress {
		coins, gasCoin, err = allCoins.PickSUICoinsWithGas(amountBig, suiGasBudget, types.PickSmaller)
		if err != nil {
			return nil, nil, err
		}
	} else {
		coins, err = allCoins.PickCoins(amountBig, types.PickSmaller)
		if err != nil {
			return nil, nil, err
		}
		suiCoins, err := cli.GetSuiCoinsOwnedByAddress(ctx, *addressHex)
		if err != nil {
			return nil, nil, err
		}
		amountU64, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, nil, err
		}
		gasCoin, err = suiCoins.PickCoinNoLess(amountU64)
		if err != nil {
			return nil, nil, err
		}
	}
	coinObjs := make([]types.ObjectId, len(coins))
	for i := range coins {
		coinObjs[i] = coins[i].Reference().ObjectId
	}
	return coinObjs, &gasCoin.Reference().ObjectId, nil
}

func (c *suiRedPacketContract) FetchRedPacketCreationDetail(hash string) (detail *RedPacketDetail, err error) {
	defer base.CatchPanicAndMapToBasicError(&err)

	cli, err := c.chain.GetClient()
	if err != nil {
		return nil, err
	}
	resp, err := cli.GetTransactionBlock(context.Background(), hash, types.SuiTransactionBlockResponseOptions{
		ShowInput:   true,
		ShowEffects: true,
		ShowEvents:  true,
	})
	if err != nil {
		return nil, err
	}
	baseTransaction, err := toSuiBaseTransaction(hash, resp)
	if err != nil {
		return nil, err
	}

	// inputs := resp.Transaction.Data.Transaction.Data.ProgrammableTransaction.Inputs
	// println(inputs)
	typeArgs := ""

	coinInfo, err := cli.GetCoinMetadata(context.Background(), typeArgs)
	if err != nil {
		return nil, err
	}

	coinAmount, err := getAmountBySuiEvents(resp.Events)
	if err != nil {
		return nil, err
	}

	detail = &RedPacketDetail{
		TransactionDetail: baseTransaction,
		AmountName:        coinInfo.Name,
		AmountDecimal:     int16(coinInfo.Decimals),
		RedPacketAmount:   strconv.FormatUint(coinAmount, 10),
		ChainName:         ChainTypeSui,
	}
	return detail, nil
}

func (c *suiRedPacketContract) EstimateFee(rpa *RedPacketAction) (string, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return "", errors.New("invalid create params")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return "", err
		}
		total := calcTotal(amount, 250)
		return strconv.FormatUint(total-amount, 10), nil
	default:
		return "", errors.New("method invalid")
	}
}

func (c *suiRedPacketContract) EstimateGasFee(account base.Account, rpa *RedPacketAction) (string, error) {
	tx, err := c.createTx(account, rpa)
	if err != nil {
		return "", err
	}
	fee, err := c.chain.EstimateGasFee(tx)
	if err != nil {
		return "", err
	}
	return fee.Value, nil
}

func getAmountBySuiEvents(events []types.SuiEvent) (uint64, error) {
	for _, event := range events {
		if !strings.Contains(event.Type, "RedPacketEvent") {
			continue
		}
		fields := event.ParsedJson.(map[string]interface{})
		remainBalance, err := strconv.ParseUint(fields["remain_balance"].(string), 10, 64)
		if err != nil {
			return 0, err
		}
		return remainBalance, nil
	}
	return 0, errors.New("not found RedPacketEvent")
}

func toSuiBaseTransaction(hash string, resp *types.SuiTransactionBlockResponse) (*base.TransactionDetail, error) {
	var total uint64

	inputs := resp.Transaction.Data.Transaction.Data.ProgrammableTransaction.Inputs
	println(inputs)

	toAddress := "" // todo

	detail := &base.TransactionDetail{
		HashString:      hash,
		FromAddress:     resp.Transaction.Data.Sender.String(),
		ToAddress:       toAddress,
		Amount:          strconv.FormatUint(total, 10),
		EstimateFees:    strconv.FormatUint(resp.Effects.GasFee(), 10),
		FinishTimestamp: int64(*resp.TimestampMs / 1000),
	}
	status := resp.Effects.Status
	if status.Status == types.ExecutionStatusSuccess {
		detail.Status = base.TransactionStatusSuccess
	} else {
		detail.Status = base.TransactionStatusFailure
		detail.FailureMessage = status.Error
	}
	return detail, nil
}
