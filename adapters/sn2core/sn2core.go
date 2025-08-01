package sn2core

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptBlock(response *starknet.Block, sig *starknet.Signature) (*core.Block, error) {
	if response == nil {
		return nil, errors.New("nil client block")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	for i, txn := range response.Transactions {
		var err error
		txns[i], err = AdaptTransaction(txn)
		if err != nil {
			return nil, err
		}
	}

	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	eventCount := uint64(0)
	for i, receipt := range response.Receipts {
		receipts[i] = AdaptTransactionReceipt(receipt)
		eventCount += uint64(len(receipt.Events))
	}

	sigs := [][]*felt.Felt{}
	if sig != nil {
		sigs = append(sigs, sig.Signature)
	}

	return &core.Block{
		Header: &core.Header{
			Hash:             response.Hash,
			ParentHash:       response.ParentHash,
			Number:           response.Number,
			GlobalStateRoot:  response.StateRoot,
			Timestamp:        response.Timestamp,
			ProtocolVersion:  response.Version,
			SequencerAddress: response.SequencerAddress,
			TransactionCount: uint64(len(response.Transactions)),
			EventCount:       eventCount,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    response.L1GasPriceETH(),
			L1GasPriceSTRK:   response.L1GasPriceSTRK(),
			L1DAMode:         core.L1DAMode(response.L1DAMode),
			L1DataGasPrice:   (*core.GasPrice)(response.L1DataGasPrice),
			L2GasPrice:       (*core.GasPrice)(response.L2GasPrice),
			Signatures:       sigs,
		},
		Transactions: txns,
		Receipts:     receipts,
	}, nil
}

func AdaptTransactionReceipt(response *starknet.TransactionReceipt) *core.TransactionReceipt {
	if response == nil {
		return nil
	}

	return &core.TransactionReceipt{
		FeeUnit:            0, // todo(kirill) recheck
		Fee:                response.ActualFee,
		TransactionHash:    response.TransactionHash,
		Events:             utils.Map(utils.NonNilSlice(response.Events), AdaptEvent),
		ExecutionResources: AdaptExecutionResources(response.ExecutionResources),
		L1ToL2Message:      AdaptL1ToL2Message(response.L1ToL2Message),
		L2ToL1Message:      utils.Map(utils.NonNilSlice(response.L2ToL1Message), AdaptL2ToL1Message),
		Reverted:           response.ExecutionStatus == starknet.Reverted,
		RevertReason:       response.RevertError,
	}
}

func adaptGasConsumed(response *starknet.GasConsumed) *core.GasConsumed {
	if response == nil {
		return nil
	}

	return &core.GasConsumed{
		L1Gas:     response.L1Gas,
		L1DataGas: response.L1DataGas,
		L2Gas:     response.L2Gas,
	}
}

func AdaptEvent(response *starknet.Event) *core.Event {
	if response == nil {
		return nil
	}

	return &core.Event{
		Data: response.Data,
		From: response.From,
		Keys: response.Keys,
	}
}

func AdaptExecutionResources(response *starknet.ExecutionResources) *core.ExecutionResources {
	if response == nil {
		return nil
	}

	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter(response.BuiltinInstanceCounter),
		MemoryHoles:            response.MemoryHoles,
		Steps:                  response.Steps,
		DataAvailability:       (*core.DataAvailability)(response.DataAvailability),
		TotalGasConsumed:       adaptGasConsumed(response.TotalGasConsumed),
	}
}

func AdaptL1ToL2Message(response *starknet.L1ToL2Message) *core.L1ToL2Message {
	if response == nil {
		return nil
	}

	return &core.L1ToL2Message{
		From:     common.HexToAddress(response.From),
		Nonce:    response.Nonce,
		Payload:  response.Payload,
		Selector: response.Selector,
		To:       response.To,
	}
}

func AdaptL2ToL1Message(response *starknet.L2ToL1Message) *core.L2ToL1Message {
	if response == nil {
		return nil
	}

	return &core.L2ToL1Message{
		From:    response.From,
		Payload: response.Payload,
		To:      common.HexToAddress(response.To),
	}
}

func AdaptTransaction(transaction *starknet.Transaction) (core.Transaction, error) {
	txType := transaction.Type
	switch txType {
	case starknet.TxnDeclare:
		return AdaptDeclareTransaction(transaction), nil
	case starknet.TxnDeploy:
		return AdaptDeployTransaction(transaction), nil
	case starknet.TxnInvoke:
		return AdaptInvokeTransaction(transaction), nil
	case starknet.TxnDeployAccount:
		return AdaptDeployAccountTransaction(transaction), nil
	case starknet.TxnL1Handler:
		return AdaptL1HandlerTransaction(transaction), nil
	default:
		return nil, fmt.Errorf("unknown transaction type %q", txType)
	}
}

func AdaptDeclareTransaction(t *starknet.Transaction) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       t.Hash,
		SenderAddress:         t.SenderAddress,
		MaxFee:                t.MaxFee,
		TransactionSignature:  *t.Signature,
		Nonce:                 t.Nonce,
		Version:               (*core.TransactionVersion)(t.Version),
		ClassHash:             t.ClassHash,
		CompiledClassHash:     t.CompiledClassHash,
		ResourceBounds:        adaptResourceBounds(t.ResourceBounds),
		Tip:                   safeFeltToUint64(t.Tip),
		PaymasterData:         utils.DerefSlice(t.PaymasterData),
		AccountDeploymentData: utils.DerefSlice(t.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func adaptDataAvailabilityMode(mode *starknet.DataAvailabilityMode) core.DataAvailabilityMode {
	if mode == nil {
		return core.DAModeL1
	}
	return core.DataAvailabilityMode(*mode)
}

func adaptResourceBounds(rb *map[starknet.Resource]starknet.ResourceBounds) map[core.Resource]core.ResourceBounds { //nolint:gocritic
	if rb == nil {
		return nil
	}
	coreBounds := make(map[core.Resource]core.ResourceBounds, len(*rb))
	for resource, bounds := range *rb {
		coreBounds[core.Resource(resource)] = core.ResourceBounds{
			MaxAmount:       bounds.MaxAmount.Uint64(),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}
	return coreBounds
}

func AdaptDeployTransaction(t *starknet.Transaction) *core.DeployTransaction {
	if t.ContractAddress == nil {
		t.ContractAddress = core.ContractAddress(&felt.Zero, t.ClassHash, t.ContractAddressSalt, *t.ConstructorCallData)
	}
	return &core.DeployTransaction{
		TransactionHash:     t.Hash,
		ContractAddressSalt: t.ContractAddressSalt,
		ContractAddress:     t.ContractAddress,
		ClassHash:           t.ClassHash,
		ConstructorCallData: *t.ConstructorCallData,
		Version:             (*core.TransactionVersion)(t.Version),
	}
}

func AdaptInvokeTransaction(t *starknet.Transaction) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash:       t.Hash,
		ContractAddress:       t.ContractAddress,
		EntryPointSelector:    t.EntryPointSelector,
		Nonce:                 t.Nonce,
		CallData:              *t.CallData,
		TransactionSignature:  *t.Signature,
		MaxFee:                t.MaxFee,
		Version:               (*core.TransactionVersion)(t.Version),
		SenderAddress:         t.SenderAddress,
		ResourceBounds:        adaptResourceBounds(t.ResourceBounds),
		Tip:                   safeFeltToUint64(t.Tip),
		PaymasterData:         utils.DerefSlice(t.PaymasterData),
		AccountDeploymentData: utils.DerefSlice(t.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func AdaptL1HandlerTransaction(t *starknet.Transaction) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		TransactionHash:    t.Hash,
		ContractAddress:    t.ContractAddress,
		EntryPointSelector: t.EntryPointSelector,
		Nonce:              t.Nonce,
		CallData:           *t.CallData,
		Version:            (*core.TransactionVersion)(t.Version),
	}
}

func AdaptDeployAccountTransaction(t *starknet.Transaction) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		DeployTransaction:    *AdaptDeployTransaction(t),
		MaxFee:               t.MaxFee,
		TransactionSignature: *t.Signature,
		Nonce:                t.Nonce,
		ResourceBounds:       adaptResourceBounds(t.ResourceBounds),
		Tip:                  safeFeltToUint64(t.Tip),
		PaymasterData:        utils.DerefSlice(t.PaymasterData),
		NonceDAMode:          adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:            adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func AdaptCairo1Class(response *starknet.SierraDefinition, compiledClass *starknet.CompiledClass) (*core.Cairo1Class, error) {
	var err error

	// TODO: what's the absolute minimum size of a Sierra Definition?
	// A Sierra program size should be at least 3 to contain the version or 1 if it's version is 0.1.0
	if len(response.Program) < 3 && (len(response.Program) == 0 || !response.Program[0].Equal(&core.SierraVersion010)) {
		return nil, errors.New("sierra program size is too small")
	}

	coreCompiledClass, err := AdaptCompiledClass(compiledClass)
	if err != nil {
		return nil, err
	}

	adapt := func(ep *starknet.SierraEntryPoint) core.SierraEntryPoint {
		return core.SierraEntryPoint{Index: ep.Index, Selector: ep.Selector}
	}

	return &core.Cairo1Class{
		SemanticVersion: response.Version,
		Program:         response.Program,
		ProgramHash:     crypto.PoseidonArray(response.Program...),

		Abi:     response.Abi,
		AbiHash: crypto.StarknetKeccak([]byte(response.Abi)),

		Compiled: coreCompiledClass,

		EntryPoints: struct {
			Constructor []core.SierraEntryPoint
			External    []core.SierraEntryPoint
			L1Handler   []core.SierraEntryPoint
		}{
			Constructor: utils.MapByRef(utils.NonNilSlice(response.EntryPoints.Constructor), adapt),
			External:    utils.MapByRef(utils.NonNilSlice(response.EntryPoints.External), adapt),
			L1Handler:   utils.MapByRef(utils.NonNilSlice(response.EntryPoints.L1Handler), adapt),
		},
	}, nil
}

func AdaptCompiledClass(compiledClass *starknet.CompiledClass) (*core.CompiledClass, error) {
	if compiledClass == nil {
		return nil, nil
	}

	var compiled core.CompiledClass
	compiled.Bytecode = compiledClass.Bytecode
	compiled.PythonicHints = compiledClass.PythonicHints
	compiled.CompilerVersion = compiledClass.CompilerVersion
	compiled.Hints = compiledClass.Hints
	compiled.BytecodeSegmentLengths = AdaptSegmentLengths(compiledClass.BytecodeSegmentLengths)

	var ok bool
	compiled.Prime, ok = new(big.Int).SetString(compiledClass.Prime, 0)
	if !ok {
		return nil, fmt.Errorf("couldn't convert prime value to big.Int: %d", compiled.Prime)
	}

	entryPoints := compiledClass.EntryPoints
	compiled.External = utils.Map(entryPoints.External, adaptCompiledEntryPoint)
	compiled.L1Handler = utils.Map(entryPoints.L1Handler, adaptCompiledEntryPoint)
	compiled.Constructor = utils.Map(entryPoints.Constructor, adaptCompiledEntryPoint)

	return &compiled, nil
}

func AdaptSegmentLengths(l starknet.SegmentLengths) core.SegmentLengths {
	return core.SegmentLengths{
		Length:   l.Length,
		Children: utils.Map(l.Children, AdaptSegmentLengths),
	}
}

func AdaptCairo0Class(response *starknet.Cairo0Definition) (core.Class, error) {
	class := new(core.Cairo0Class)
	class.Abi = response.Abi

	adapt := func(ep starknet.EntryPoint) core.EntryPoint {
		return core.EntryPoint{Selector: ep.Selector, Offset: ep.Offset}
	}

	class.Externals = utils.Map(utils.NonNilSlice(response.EntryPoints.External), adapt)
	class.L1Handlers = utils.Map(utils.NonNilSlice(response.EntryPoints.L1Handler), adapt)
	class.Constructors = utils.Map(utils.NonNilSlice(response.EntryPoints.Constructor), adapt)

	var err error
	class.Program, err = utils.Gzip64Encode(response.Program)
	if err != nil {
		return nil, err
	}

	return class, nil
}

func AdaptStateUpdate(response *starknet.StateUpdate) (*core.StateUpdate, error) {
	stateDiff, err := AdaptStateDiff(&response.StateDiff)
	if err != nil {
		return nil, err
	}

	return &core.StateUpdate{
		BlockHash: response.BlockHash,
		NewRoot:   response.NewRoot,
		OldRoot:   response.OldRoot,
		StateDiff: stateDiff,
	}, nil
}

func AdaptStateDiff(response *starknet.StateDiff) (*core.StateDiff, error) {
	stateDiff := new(core.StateDiff)
	stateDiff.DeclaredV0Classes = response.OldDeclaredContracts

	stateDiff.DeclaredV1Classes = make(map[felt.Felt]*felt.Felt, len(response.DeclaredClasses))
	for _, declaredV1Class := range response.DeclaredClasses {
		stateDiff.DeclaredV1Classes[*declaredV1Class.ClassHash] = declaredV1Class.CompiledClassHash
	}

	stateDiff.ReplacedClasses = make(map[felt.Felt]*felt.Felt, len(response.ReplacedClasses))
	for _, replacedClass := range response.ReplacedClasses {
		stateDiff.ReplacedClasses[*replacedClass.Address] = replacedClass.ClassHash
	}

	stateDiff.DeployedContracts = make(map[felt.Felt]*felt.Felt, len(response.DeployedContracts))
	for _, deployedContract := range response.DeployedContracts {
		stateDiff.DeployedContracts[*deployedContract.Address] = deployedContract.ClassHash
	}

	stateDiff.Nonces = make(map[felt.Felt]*felt.Felt, len(response.Nonces))
	for addrStr, nonce := range response.Nonces {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}
		stateDiff.Nonces[*addr] = nonce
	}

	stateDiff.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(response.StorageDiffs))
	for addrStr, diffs := range response.StorageDiffs {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}

		stateDiff.StorageDiffs[*addr] = make(map[felt.Felt]*felt.Felt)
		for _, diff := range diffs {
			stateDiff.StorageDiffs[*addr][*diff.Key] = diff.Value
		}
	}

	return stateDiff, nil
}

func AdaptPreConfirmedBlock(response *starknet.PreConfirmedBlock, number uint64) (core.PreConfirmed, error) {
	if response == nil {
		return core.PreConfirmed{}, errors.New("nil preconfirmed block")
	}

	if response.Status != "PRE_CONFIRMED" {
		return core.PreConfirmed{}, errors.New("invalid status for pre_confirmed block")
	}

	var adaptedStateDiff *core.StateDiff
	var err error

	txStateDiffs := make([]*core.StateDiff, 0, len(response.TransactionStateDiffs))
	for _, stateDiff := range response.TransactionStateDiffs {
		if stateDiff == nil {
			break
		}

		if adaptedStateDiff, err = AdaptStateDiff(stateDiff); err != nil {
			return core.PreConfirmed{}, err
		}
		txStateDiffs = append(txStateDiffs, adaptedStateDiff)
	}

	preConfirmedTxCount := len(txStateDiffs)

	txns := make([]core.Transaction, preConfirmedTxCount)
	for i := range preConfirmedTxCount {
		txns[i], err = AdaptTransaction(&response.Transactions[i])
		if err != nil {
			return core.PreConfirmed{}, err
		}
	}

	rawTxCount := len(response.Transactions)
	candidateTxs := make([]core.Transaction, rawTxCount-preConfirmedTxCount)

	for i := range rawTxCount - preConfirmedTxCount {
		candidateTxs[i], err = AdaptTransaction(&response.Transactions[preConfirmedTxCount+i])
		if err != nil {
			return core.PreConfirmed{}, err
		}
	}

	receipts := make([]*core.TransactionReceipt, preConfirmedTxCount)
	eventCount := uint64(0)
	for i, receipt := range response.Receipts[:preConfirmedTxCount] {
		receipts[i] = AdaptTransactionReceipt(receipt)
		eventCount += uint64(len(receipt.Events))
	}

	// Squash per-tx state updates
	stateDiff := core.EmptyStateDiff()
	for _, txStateDiff := range txStateDiffs {
		stateDiff.Merge(txStateDiff)
	}

	stateUpdate := core.StateUpdate{
		BlockHash: nil,
		NewRoot:   nil,
		// Must be set to previous global state root, when have access to latest header
		OldRoot:   nil,
		StateDiff: &stateDiff,
	}

	adaptedBlock := &core.Block{
		// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L1636
		Header: &core.Header{
			Number:           number,
			SequencerAddress: response.SequencerAddress,
			// Not required in spec but useful
			TransactionCount: uint64(len(txns)),
			// Not required in spec but useful
			EventCount:      eventCount,
			Timestamp:       response.Timestamp,
			ProtocolVersion: response.Version,
			// Not required in spec but useful
			EventsBloom:    core.EventsBloom(receipts),
			L1GasPriceETH:  response.L1GasPrice.PriceInWei,
			L1GasPriceSTRK: response.L1GasPrice.PriceInFri,
			L1DAMode:       core.L1DAMode(response.L1DAMode),
			L1DataGasPrice: (*core.GasPrice)(response.L1DataGasPrice),
			L2GasPrice:     (*core.GasPrice)(response.L2GasPrice),
			// Following fields are nil for pre_confirmed block
			Hash:            nil,
			ParentHash:      nil,
			GlobalStateRoot: nil,
			Signatures:      nil,
		},
		Transactions: txns,
		Receipts:     receipts,
	}
	return core.NewPreConfirmed(adaptedBlock, &stateUpdate, txStateDiffs, candidateTxs), nil
}

func safeFeltToUint64(f *felt.Felt) uint64 {
	if f != nil {
		return f.Uint64()
	}
	return 0
}

func adaptCompiledEntryPoint(entryPoint starknet.CompiledEntryPoint) core.CompiledEntryPoint {
	return core.CompiledEntryPoint{
		Offset:   entryPoint.Offset,
		Selector: entryPoint.Selector,
		Builtins: entryPoint.Builtins,
	}
}
