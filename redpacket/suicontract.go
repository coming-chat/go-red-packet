package redpacket

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/coming-chat/go-sui/v2/lib"
	"github.com/coming-chat/go-sui/v2/move_types"
	"github.com/coming-chat/go-sui/v2/sui_types"
	"github.com/coming-chat/go-sui/v2/types"
	"github.com/coming-chat/wallet-SDK/core/base"
	"github.com/coming-chat/wallet-SDK/core/sui"
	"github.com/fardream/go-bcs/bcs"
)

const (
	SuiDecimal = 9

	suiPackage     = "red_packet"
	suiCoinAddress = "0x2::sui::SUI"

	suiFeePoint = 250
)

type suiRedPacketContract struct {
	chain        *sui.Chain
	address      string
	packageIdHex sui_types.SuiAddress
	configHex    sui_types.ObjectID
}

func NewSuiRedPacketContract(chain *sui.Chain, contractAddress string, config *ContractConfig) (RedPacketContract, error) {
	address := "0x" + strings.TrimPrefix(contractAddress, "0x")
	pkgId, err := sui_types.NewAddressFromHex(address)
	if err != nil {
		return nil, err
	}
	configHex, err := sui_types.NewObjectIdFromHex(config.SuiConfigAddress)
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

	tokenAddress := rpa.TokenAddress()
	resourceType, err := types.NewResourceType(tokenAddress)
	if err != nil {
		return nil, err
	}
	typeTag := move_types.StructTag{
		Address: *resourceType.Address,
		Module:  move_types.Identifier(resourceType.ModuleName),
		Name:    move_types.Identifier(resourceType.FuncName),
	}
	configObject, err := cli.GetObject(context.Background(), c.configHex, &types.SuiObjectDataOptions{
		ShowOwner: true,
	})
	if err != nil {
		return nil, err
	}
	if configObject.Data == nil || configObject.Data.Owner == nil || configObject.Data.Owner.Shared == nil || configObject.Data.Owner.Shared.InitialSharedVersion == nil {
		return nil, errors.New("invalid shared config address")
	}
	configCallArg := sui_types.ObjectArg{SharedObject: &struct {
		Id                   move_types.AccountAddress
		InitialSharedVersion uint64
		Mutable              bool
	}{
		Id:                   c.configHex,
		InitialSharedVersion: *configObject.Data.Owner.Shared.InitialSharedVersion,
		Mutable:              true,
	}}

	sender, _ := sui_types.NewAddressFromHex(account.Address())
	switch rpa.Method {
	case RPAMethodCreate:
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amount params is not uint64")
		}
		amountTotal := calcTotal(amount, suiFeePoint)

		coins, err := cli.GetCoins(context.Background(), *sender, &tokenAddress, nil, 100)
		if err != nil {
			return nil, err
		}
		var pickedCoins *types.PickedCoins
		var pickedGasCoins *types.PickedCoins
		if tokenAddress == suiCoinAddress {
			pickedCoins = nil
			pickedGasCoins, err = types.PickupCoins(coins, *big.NewInt(0).SetUint64(amountTotal), sui.MaxGasForPay, 100, 0)
			if err != nil {
				return nil, err
			}
		} else {
			pickedCoins, err = types.PickupCoins(coins, *big.NewInt(0).SetUint64(amountTotal), 0, 100, 0)
			if err != nil {
				return nil, err
			}
			pickedGasCoins, err = c.chain.PickGasCoins(*sender, sui.MaxGasForPay)
			if err != nil {
				return nil, err
			}
		}

		maxGasBudget := base.Min(pickedGasCoins.SuggestMaxGasBudget(), sui.MaxGasForPay)
		gasPrice, _ := c.chain.CachedGasPrice()

		return c.chain.EstimateTransactionFeeAndRebuildTransactionBCS(maxGasBudget, func(gasBudget uint64) (*sui.Transaction, error) {
			ptb := sui_types.NewProgrammableTransactionBuilder()
			var arg0, arg1, arg2, arg3 sui_types.Argument
			amtArg, err := ptb.Pure(amountTotal)
			if err != nil {
				return nil, err
			}
			if tokenAddress == suiCoinAddress {
				arg := ptb.Command(
					sui_types.Command{
						SplitCoins: &struct {
							Argument  sui_types.Argument
							Arguments []sui_types.Argument
						}{
							Argument:  sui_types.Argument{GasCoin: &lib.EmptyEnum{}},
							Arguments: []sui_types.Argument{amtArg},
						},
					},
				)
				arg1 = ptb.Command(
					sui_types.Command{
						MakeMoveVec: &struct {
							TypeTag   *move_types.TypeTag `bcs:"optional"`
							Arguments []sui_types.Argument
						}{TypeTag: nil, Arguments: []sui_types.Argument{arg}},
					},
				)
			} else {
				coinArgs := make([]sui_types.ObjectArg, len(pickedCoins.Coins))
				for idx, coin := range pickedCoins.Coins {
					coinArgs[idx] = sui_types.ObjectArg{
						ImmOrOwnedObject: coin.Reference(),
					}
				}
				arg1, err = ptb.MakeObjList(coinArgs)
				if err != nil {
					return nil, err
				}
			}

			arg0, err = ptb.Obj(configCallArg)
			if err != nil {
				return nil, err
			}
			arg2, err = ptb.Pure(uint64(rpa.CreateParams.Count))
			if err != nil {
				return nil, err
			}
			arg3 = amtArg

			ptb.Command(
				sui_types.Command{
					MoveCall: &sui_types.ProgrammableMoveCall{
						Package:  c.packageIdHex,
						Module:   move_types.Identifier(suiPackage),
						Function: move_types.Identifier("create"),
						TypeArguments: []move_types.TypeTag{
							{Struct: &typeTag},
						},
						Arguments: []sui_types.Argument{
							arg0, arg1, arg2, arg3,
						},
					},
				},
			)
			pt := ptb.Finish()
			tx := sui_types.NewProgrammable(*sender, pickedGasCoins.CoinRefs(), pt, gasBudget, gasPrice)
			txBytes, err := bcs.Marshal(tx)
			if err != nil {
				return nil, err
			}
			return &sui.Transaction{TxnBytes: txBytes}, nil
		})
	case RPAMethodOpen:
		if len(rpa.OpenParams.PacketObjectId) == 0 {
			return nil, errors.New("invalid redPacketObjectId")
		}
		redPacketObjectId, err := lib.NewHexData(rpa.OpenParams.PacketObjectId)
		if err != nil {
			return nil, err
		}
		addresses := make([]*lib.HexData, len(rpa.OpenParams.Addresses))
		for i := range rpa.OpenParams.Addresses {
			addresses[i], err = lib.NewHexData(rpa.OpenParams.Addresses[i])
			if err != nil {
				return nil, err
			}
		}
		args := []interface{}{
			redPacketObjectId,
			addresses,
			rpa.OpenParams.Amounts,
		}
		return c.chain.BaseMoveCall(
			account.Address(),
			c.packageIdHex.String(),
			suiPackage,
			"open",
			[]string{rpa.OpenParams.TokenAddress},
			args,
			0,
		)
	case RPAMethodClose:
		if len(rpa.CloseParams.PacketObjectId) == 0 {
			return nil, errors.New("invalid redPacketObjectId")
		}
		packetId, err := lib.NewHexData(rpa.CloseParams.PacketObjectId)
		if err != nil {
			return nil, err
		}
		args := []interface{}{
			packetId,
		}
		return c.chain.BaseMoveCall(
			account.Address(),
			c.packageIdHex.String(),
			suiPackage,
			"close",
			[]string{rpa.CloseParams.TokenAddress},
			args,
			0,
		)
	default:
		return nil, fmt.Errorf("unsopported red packet method %s", rpa.Method)
	}
}

func (c *suiRedPacketContract) FetchRedPacketCreationDetail(hash string) (detail *RedPacketDetail, err error) {
	defer base.CatchPanicAndMapToBasicError(&err)

	cli, err := c.chain.Client()
	if err != nil {
		return nil, err
	}
	digest, err := sui_types.NewDigest(hash)
	if err != nil {
		return nil, err
	}
	resp, err := cli.GetTransactionBlock(context.Background(), *digest, types.SuiTransactionBlockResponseOptions{
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
	feeString := strconv.FormatInt(tx.EstimateGasFee, 10)
	return feeString, nil
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
