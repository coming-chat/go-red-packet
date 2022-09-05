package redpacket

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coming-chat/go-aptos/aptostypes"
	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
	"github.com/coming-chat/wallet-SDK/core/aptos"
	"github.com/coming-chat/wallet-SDK/core/base"
)

const (
	AptosName    = aptos.AptosName
	AptosSymbol  = aptos.AptosSymbol
	AptosDecimal = aptos.AptosDecimal

	MaxGasAmount = 1000
	GasPrice     = 1

	createABI = "0106637265617465b39c45e31d1429218aeb3590e2a046edae9303fbbc3ef6a065384569cfd818810a7265645f7061636b657400000205636f756e74020d746f74616c5f62616c616e636502"
	openABI   = "01046f70656eb39c45e31d1429218aeb3590e2a046edae9303fbbc3ef6a065384569cfd818810a7265645f7061636b6574000003026964020e6c75636b795f6163636f756e747306040862616c616e6365730602"
	closeABI  = "0105636c6f7365b39c45e31d1429218aeb3590e2a046edae9303fbbc3ef6a065384569cfd818810a7265645f7061636b657400000102696402"
)

// aptosRedPacketContract implement RedPacketContract interface
type aptosRedPacketContract struct {
	chain   aptos.IChain
	address string
	abi     *transactionbuilder.TransactionBuilderABI
}

func NewAptosRedPacketContract(chain aptos.IChain, contractAddress string) RedPacketContract {
	abiBytes := make([][]byte, 0)
	abiStrs := []string{createABI, openABI, closeABI}
	for _, s := range abiStrs {
		bs, _ := hex.DecodeString(s)
		abiBytes = append(abiBytes, bs)
	}
	redpacketAbi, err := transactionbuilder.NewTransactionBuilderABI(abiBytes)
	if err != nil {
		return nil
	}

	return &aptosRedPacketContract{
		chain:   chain,
		address: "0x" + strings.TrimPrefix(contractAddress, "0x"),
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
		feePoint, err := contract.getFeePoint()
		if err != nil {
			return "", err
		}
		total := calcTotal(amount, uint64(feePoint))
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
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	gasFee, err := contract.chain.EstimatePayloadGasFee(acocunt, data)
	if err != nil {
		return "", err
	}
	return gasFee.Value, nil
}

// getFeePoint get fee_point from contract by resouce
// when api support call move public function, should not use resouce
func (contract *aptosRedPacketContract) getFeePoint() (uint64, error) {
	client, err := contract.chain.GetClient()
	if err != nil {
		return 0, err
	}
	resource, err := client.GetAccountResource(contract.address, contract.address+"::red_packet::RedPackets", 0)
	if err != nil {
		return 0, err
	}
	config, _ := resource.Data["config"].(map[string]interface{})
	feePoint, _ := config["fee_point"].(float64)
	return uint64(feePoint), nil
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

	redPacketDetail := &RedPacketDetail{
		TransactionDetail: baseTransaction,
		AmountName:        AptosName,
		AmountDecimal:     AptosDecimal,
	}

	if len(transaction.Payload.Arguments) < 2 {
		return redPacketDetail, newRedPacketDataError(fmt.Sprintf("invalid payload arguments, len %d", len(transaction.Payload.Arguments)))
	}
	baseTransaction.Amount = transaction.Payload.Arguments[1].(string)

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
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return contract.chain.SubmitTransactionPayload(account, data)
}

func (contract *aptosRedPacketContract) createPayload(rpa *RedPacketAction) (*aptostypes.Payload, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return nil, fmt.Errorf("create params is nil")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amount params is not uint64")
		}
		feePoint, err := contract.getFeePoint()
		if err != nil {
			return nil, err
		}
		amountTotal := calcTotal(amount, feePoint)
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::create",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.CreateParams.Count), 10),
				strconv.FormatUint(amountTotal, 10),
			},
		}, nil
	case RPAMethodOpen:
		if nil == rpa.OpenParams {
			return nil, fmt.Errorf("open params is nil")
		}
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::open",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.OpenParams.PacketId), 10),
				rpa.OpenParams.Addresses,
				rpa.OpenParams.Amounts,
			},
		}, nil
	case RPAMethodClose:
		if nil == rpa.CloseParams {
			return nil, fmt.Errorf("close params is nil")
		}
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::close",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.CloseParams.PacketId), 10),
			},
		}, nil
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
