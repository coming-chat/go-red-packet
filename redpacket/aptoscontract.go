package redpacket

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coming-chat/go-aptos/aptostypes"
	txbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	"github.com/coming-chat/lcs"
	"github.com/coming-chat/wallet-SDK/core/aptos"
	"github.com/coming-chat/wallet-SDK/core/base"
)

const (
	AptosName    = aptos.AptosName
	AptosSymbol  = aptos.AptosSymbol
	AptosDecimal = 8

	createABIFormat = "0106637265617465%s0a7265645f7061636b6574322063616c6c20627920616e796f6e6520696e20636f6d696e67636861740a20637265617465206120726564207061636b65740109636f696e5f74797065030d68616e646c65725f696e6465780205636f756e74020d746f74616c5f62616c616e636502"
	openABIFormat   = "01046f70656e%s0a7265645f7061636b65748201206f6666636861696e20636865636b0a20312e2064656475706c6963617465206c75636b79206163636f756e74730a20322e20636865636b206c75636b79206163636f756e74206973206578736973740a20332e20636865636b20746f74616c2062616c616e63650a2063616c6c20627920636f6d696e67636861742061646d696e0109636f696e5f74797065040d68616e646c65725f696e64657802026964020e6c75636b795f6163636f756e747306040862616c616e6365730602"
	closeABIFormat  = "0105636c6f7365%s0a7265645f7061636b65742d2063616c6c20627920636f6d696e67636861742061646d696e0a20636c6f7365206120726564207061636b65740109636f696e5f74797065020d68616e646c65725f696e6465780202696402"
)

type tokenHandler struct {
	CoinType     string
	HandlerIndex uint64
	FeePoint     uint64
}

// aptosRedPacketContract implement RedPacketContract interface
type aptosRedPacketContract struct {
	chain   aptos.IChain
	address string
	abi     *txbuilder.TransactionBuilderABI
}

func NewAptosRedPacketContract(chain aptos.IChain, contractAddress string) RedPacketContract {
	contractAddressWithOurPrefix := strings.TrimPrefix(contractAddress, "0x")
	abiBytes := make([][]byte, 0)
	abiStrFormats := []string{createABIFormat, openABIFormat, closeABIFormat}
	for _, s := range abiStrFormats {
		bs, _ := hex.DecodeString(fmt.Sprintf(s, contractAddressWithOurPrefix))
		abiBytes = append(abiBytes, bs)
	}
	redpacketAbi, err := txbuilder.NewTransactionBuilderABI(abiBytes)
	if err != nil {
		return nil
	}

	return &aptosRedPacketContract{
		chain:   chain,
		address: "0x" + contractAddressWithOurPrefix,
		abi:     redpacketAbi,
	}
}

func (contract *aptosRedPacketContract) EstimateFee(rpa *RedPacketAction) (string, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return "", errors.New("invalid create params")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return "", err
		}
		handler, err := contract.getTokenHandler(rpa.CreateParams.TokenAddress)
		if err != nil {
			return "", err
		}
		total := calcTotal(amount, uint64(handler.FeePoint))
		return strconv.FormatUint(total-amount, 10), nil
	default:
		return "", errors.New("method invalid")
	}
}

func (contract *aptosRedPacketContract) EstimateGasFee(acocunt base.Account, rpa *RedPacketAction) (string, error) {
	payload, err := contract.createPayload(rpa)
	if err != nil {
		return "", err
	}
	data, err := lcs.Marshal(payload)
	if err != nil {
		return "", err
	}
	gasFee, err := contract.chain.EstimatePayloadGasFeeBCS(acocunt, data)
	if err != nil {
		return "", err
	}
	return gasFee.Value, nil
}

// getFeePoint get fee_point from contract by resouce
// when api support call move public function, should not use resouce
func (contract *aptosRedPacketContract) getTokenHandler(tokenAddress string) (tokenHandler, error) {
	client, err := contract.chain.GetClient()
	if err != nil {
		return tokenHandler{}, errors.New("get client failed")
	}
	resource, err := client.GetAccountResource(contract.address, contract.address+"::red_packet::GlobalConfig", 0)
	if err != nil {
		return tokenHandler{}, errors.New("not found account resource")
	}
	handlers, _ := resource.Data["handlers"].([]interface{})
	for _, handler := range handlers {
		handlerMap, _ := handler.(map[string]interface{})
		coinType, _ := handlerMap["coin_type"].(string)
		if coinType != tokenAddress {
			continue
		}
		config, _ := handlerMap["config"].(map[string]interface{})
		feePoint, _ := config["fee_point"].(float64)
		handlerIndex, _ := strconv.ParseUint(handlerMap["handler_index"].(string), 10, 64)
		return tokenHandler{
			CoinType:     coinType,
			HandlerIndex: handlerIndex,
			FeePoint:     uint64(feePoint),
		}, nil
	}
	return tokenHandler{}, errors.New("not found token handler")
}

func (contract *aptosRedPacketContract) FetchRedPacketCreationDetail(hash string) (*RedPacketDetail, error) {
	client, err := contract.chain.GetClient()
	if err != nil {
		return nil, err
	}
	transaction, err := client.GetTransactionByHash(hash)
	if err != nil {
		var restError *aptostypes.RestError
		if errors.As(err, &restError) {
			return nil, newRedPacketDataError(restError.Message)
		}
		return nil, err
	}
	baseTransaction, err := toBaseTransaction(transaction)
	if err != nil {
		return nil, newRedPacketDataError(err.Error())
	}

	if len(transaction.Payload.TypeArguments) == 0 {
		return nil, newRedPacketDataError("invalid transaction type args")
	}

	coinInfo, err := client.GetCoinInfo(transaction.Payload.TypeArguments[0])
	if err != nil {
		return nil, err
	}

	redPacketDetail := &RedPacketDetail{
		TransactionDetail: baseTransaction,
		AmountName:        coinInfo.Name,
		AmountDecimal:     int16(coinInfo.Decimals),
	}

	if len(transaction.Payload.Arguments) < 3 {
		return redPacketDetail, newRedPacketDataError(fmt.Sprintf("invalid payload arguments, len %d", len(transaction.Payload.Arguments)))
	}
	baseTransaction.Amount = transaction.Payload.Arguments[2].(string)

	redPacketAmount := "0"

	for _, event := range transaction.Events {
		if event.Type != contract.address+"::red_packet::RedPacketEvent" {
			continue
		}
		eventData, ok := event.Data.(map[string]interface{})
		if !ok {
			return redPacketDetail, newRedPacketDataError("redpacket event data is not map[string]interface{}")
		}
		eventType, ok := eventData["event_type"].(float64)
		if !ok {
			return redPacketDetail, newRedPacketDataError("redpacket data eventType is not float64")
		}
		// 0 是 create event
		if int(eventType) != 0 {
			return redPacketDetail, newRedPacketDataError("not create event")
		}
		redPacketAmount, ok = eventData["remain_balance"].(string)
		if !ok {
			return redPacketDetail, newRedPacketDataError("redpacket data remain_balance is not string")
		}
		break
	}

	redPacketDetail.RedPacketAmount = redPacketAmount
	return redPacketDetail, nil
}

func (contract *aptosRedPacketContract) SendTransaction(account base.Account, rpa *RedPacketAction) (string, error) {
	payload, err := contract.createPayload(rpa)
	if err != nil {
		return "", err
	}
	data, err := lcs.Marshal(payload)
	if err != nil {
		return "", err
	}
	return contract.chain.SubmitTransactionPayloadBCS(account, data)
}

func (contract *aptosRedPacketContract) createPayload(rpa *RedPacketAction) (txbuilder.TransactionPayload, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return nil, fmt.Errorf("create params is nil")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amount params is not uint64")
		}
		handler, err := contract.getTokenHandler(rpa.CreateParams.TokenAddress)
		if err != nil {
			return nil, err
		}
		amountTotal := calcTotal(amount, handler.FeePoint)
		return contract.abi.BuildTransactionPayload(
			contract.address+"::red_packet::create",
			[]string{
				rpa.CreateParams.TokenAddress,
			},
			[]any{
				handler.HandlerIndex,
				uint64(rpa.CreateParams.Count),
				uint64(amountTotal),
			},
		)
	case RPAMethodOpen:
		if nil == rpa.OpenParams {
			return nil, fmt.Errorf("open params is nil")
		}
		if rpa.OpenParams.TokenAddress == "" {
			return nil, fmt.Errorf("params.TokenAddress must not empty")
		}
		amountsArr := make([]any, len(rpa.OpenParams.Amounts))
		addressList := make([]any, len(rpa.OpenParams.Addresses))
		var err error
		for i, a := range rpa.OpenParams.Amounts {
			amountsArr[i], err = strconv.ParseUint(a, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("open amounts error")
			}
			paddress, e := txbuilder.NewAccountAddressFromHex(rpa.OpenParams.Addresses[i])
			if e != nil {
				return nil, fmt.Errorf("open amounts error")
			}
			addressList[i] = *paddress
		}
		handler, err := contract.getTokenHandler(rpa.OpenParams.TokenAddress)
		if err != nil {
			return nil, err
		}
		return contract.abi.BuildTransactionPayload(
			contract.address+"::red_packet::open",
			[]string{
				rpa.OpenParams.TokenAddress,
			},
			[]any{
				handler.HandlerIndex,
				uint64(rpa.OpenParams.PacketId),
				addressList,
				amountsArr,
			},
		)
	case RPAMethodClose:
		if nil == rpa.CloseParams {
			return nil, fmt.Errorf("close params is nil")
		}
		if rpa.CloseParams.TokenAddress == "" {
			return nil, fmt.Errorf("params.TokenAddress must not empty")
		}
		handler, err := contract.getTokenHandler(rpa.CloseParams.TokenAddress)
		if err != nil {
			return nil, err
		}
		return contract.abi.BuildTransactionPayload(
			contract.address+"::red_packet::close",
			[]string{
				rpa.CloseParams.TokenAddress,
			},
			[]any{
				handler.HandlerIndex,
				uint64(rpa.CloseParams.PacketId),
			},
		)
	default:
		return nil, fmt.Errorf("unsopported red packet method %s", rpa.Method)
	}
}

// calcTotal caculate totalAmount should send, when user want create a red packet with amount
func calcTotal(amount uint64, feePoint uint64) uint64 {
	if feePoint == 0 {
		feePoint = 250
	}
	if amount < 10000 {
		return amount
	}
	fee := amount / 10000 * feePoint
	left := uint64(0)
	right := amount / feePoint
	for left <= right {
		center := (left + right) / 2
		tmpFee := center*feePoint + fee
		tmpTotal := tmpFee + amount
		tmpC := tmpTotal - tmpTotal/10000*feePoint
		if tmpC > amount {
			right = center - 1
		} else if tmpC < amount {
			left = center + 1
		} else {
			return tmpTotal
		}
	}
	return amount
}

func toBaseTransaction(transaction *aptostypes.Transaction) (*base.TransactionDetail, error) {
	if transaction.Type != aptostypes.TypeUserTransaction ||
		transaction.Payload.Type != aptostypes.EntryFunctionPayload {
		return nil, errors.New("invalid transfer transaction")
	}

	detail := &base.TransactionDetail{
		HashString:  transaction.Hash,
		FromAddress: transaction.Sender,
	}

	gasFee := transaction.GasUnitPrice * transaction.GasUsed
	detail.EstimateFees = strconv.FormatUint(gasFee, 10)

	args := transaction.Payload.Arguments
	if len(args) >= 2 {
		detail.Amount = args[1].(string)
	}

	detail.ToAddress = strings.Split(transaction.Payload.Function, "::")[0]

	if transaction.Success {
		detail.Status = base.TransactionStatusSuccess
	} else {
		detail.Status = base.TransactionStatusFailure
		detail.FailureMessage = transaction.VmStatus
	}

	timestamp := transaction.Timestamp / 1e6
	detail.FinishTimestamp = int64(timestamp)

	return detail, nil
}
