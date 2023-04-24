package redpacket

import (
	"context"
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
	suiGasBudget = sui.MaxGasBudget

	suiPackage     = "red_packet"
	suiCoinAddress = "0x2::sui::SUI"

	suiFeePoint = 250
)

var suiGasBudgetData types.SafeSuiBigInt[uint64]

func init() {
	suiGasBudgetData = types.NewSafeSuiBigInt(uint64(suiGasBudget))
}

type suiRedPacketContract struct {
	chain        *sui.Chain
	address      string
	packageIdHex types.HexData
	configHex    types.HexData
}

func NewSuiRedPacketContract(chain *sui.Chain, contractAddress string, config *ContractConfig) (RedPacketContract, error) {
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
	return c.chain.SendRawTransaction(signedTxn.Value)
}

func (c *suiRedPacketContract) createTx(account base.Account, rpa *RedPacketAction) (*sui.Transaction, error) {
	cli, err := c.chain.Client()
	if err != nil {
		return nil, err
	}
	addr, err := types.NewAddressFromHex(account.Address())
	if err != nil {
		return nil, err
	}
	switch rpa.Method {
	case RPAMethodCreate:
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amount params is not uint64")
		}
		amountTotal := calcTotal(amount, suiFeePoint)
		amountTotalStr := strconv.FormatUint(amountTotal, 10)

		coins, gas, err := c.pickCoinsAndGas(cli, account, rpa.CreateParams.TokenAddress, amountTotalStr, true)
		if err != nil {
			return nil, err
		}
		args := []interface{}{
			c.configHex,
			coins,
			strconv.Itoa(rpa.CreateParams.Count),
			amountTotalStr,
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
			suiGasBudgetData,
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
			suiGasBudgetData,
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
			suiGasBudgetData,
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

func (c *suiRedPacketContract) pickCoinsAndGas(cli *client.Client, account base.Account, token, amount string, firstTry bool) ([]types.ObjectId, *types.ObjectId, error) {
	ctx := context.Background()
	addressHex, _ := types.NewHexData(account.Address())
	amountInt, ok := big.NewInt(0).SetString(amount, 10)
	if !ok {
		return nil, nil, errors.New("amount is not a number")
	}
	allCoinsStruct, err := cli.GetCoins(ctx, *addressHex, &token, nil, 100)
	if err != nil {
		return nil, nil, err
	}
	pickedCoins, err := types.PickupCoins(allCoinsStruct, *amountInt, 100, true)
	if err != nil {
		return nil, nil, err
	}
	// gasCoin return nil, node will pick one from the signer's possession if not provided
	return pickedCoins.CoinIds(), nil, nil
}

func (c *suiRedPacketContract) FetchRedPacketCreationDetail(hash string) (detail *RedPacketDetail, err error) {
	defer base.CatchPanicAndMapToBasicError(&err)

	cli, err := c.chain.Client()
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
	coinType, baseTransaction, err := toSuiBaseTransaction(hash, resp)
	if err != nil {
		return nil, err
	}

	coinInfo, err := cli.GetCoinMetadata(context.Background(), coinType)
	if err != nil {
		if coinType == suiCoinAddress {
			coinInfo = &types.SuiCoinMetadata{
				Decimals: 9,
				Symbol:   "SUI",
				Name:     "SUI",
			}
		} else {
			return nil, err
		}
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
		total := calcTotal(amount, suiFeePoint)
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

func toSuiBaseTransaction(hash string, resp *types.SuiTransactionBlockResponse) (string, *base.TransactionDetail, error) {
	var coinType string
	if nil == resp.Transaction {
		return coinType, nil, errors.New("not found transaction")
	}
	if nil == resp.Transaction.Data.Data.V1 {
		return coinType, nil, errors.New("not programmable transaction")
	}
	programmableTransaction := resp.Transaction.Data.Data.V1.Transaction.Data.ProgrammableTransaction
	if nil == programmableTransaction {
		return coinType, nil, errors.New("not programmable transaction")
	}

	inputs := programmableTransaction.Inputs
	if len(inputs) < 4 {
		return coinType, nil, errors.New("invalid input args")
	}

	// inputs coins 如果是多个，则 inputs 数量比实际参数数量多
	// amount 参数位置在最后一个，需要从最后一个取
	inputCoinAmount := inputs[len(inputs)-1].(map[string]interface{})["value"].(string)

	var toAddress string
	for _, command := range programmableTransaction.Commands {
		moveCallCommand := command.(map[string]interface{})
		if moveCallData, ok := moveCallCommand["MoveCall"]; ok {
			moveCallMap := moveCallData.(map[string]interface{})
			toAddress = moveCallMap["package"].(string)
			typeArgs := moveCallMap["type_arguments"].([]interface{})
			if len(typeArgs) == 0 {
				return coinType, nil, errors.New("invalid type args")
			}
			coinType = typeArgs[0].(string)
		}
	}
	if toAddress == "" {
		return coinType, nil, errors.New("invalid to package address")
	}

	gasUsed := resp.Effects.Data.V1.GasUsed
	totalGas := gasUsed.ComputationCost.Uint64() + gasUsed.StorageCost.Uint64() - gasUsed.StorageRebate.Uint64()

	detail := &base.TransactionDetail{
		HashString:   hash,
		FromAddress:  resp.Transaction.Data.Data.V1.Sender.String(),
		ToAddress:    toAddress,
		Amount:       inputCoinAmount,
		EstimateFees: strconv.FormatUint(totalGas, 10),
	}
	if resp.TimestampMs != nil {
		detail.FinishTimestamp = int64(resp.TimestampMs.Uint64() / 1000)
	}
	status := resp.Effects.Data.V1.Status
	if status.Status == types.ExecutionStatusSuccess {
		detail.Status = base.TransactionStatusSuccess
	} else {
		detail.Status = base.TransactionStatusFailure
		detail.FailureMessage = status.Error
	}
	return coinType, detail, nil
}
