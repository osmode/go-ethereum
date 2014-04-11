package ethui

import (
	"bitbucket.org/kardianos/osext"
	"fmt"
	"github.com/ethereum/eth-go"
	"github.com/ethereum/eth-go/ethchain"
	"github.com/ethereum/eth-go/ethutil"
	"github.com/niemeyer/qml"
	"github.com/obscuren/mutan"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// UI Library that has some basic functionality exposed
type UiLib struct {
	engine    *qml.Engine
	eth       *eth.Ethereum
	connected bool
	assetPath string
	// The main application window
	win *qml.Window
}

func NewUiLib(engine *qml.Engine, eth *eth.Ethereum, assetPath string) *UiLib {
	if assetPath == "" {
		assetPath = DefaultAssetPath()
	}
	return &UiLib{engine: engine, eth: eth, assetPath: assetPath}
}

// Opens a QML file (external application)
func (ui *UiLib) Open(path string) {
	component, err := ui.engine.LoadFile(path[7:])
	if err != nil {
		ethutil.Config.Log.Debugln(err)
	}
	win := component.CreateWindow(nil)

	go func() {
		win.Show()
		win.Wait()
	}()
}

func (ui *UiLib) Connect(button qml.Object) {
	if !ui.connected {
		ui.eth.Start()
		ui.connected = true
		button.Set("enabled", false)
	}
}

func (ui *UiLib) ConnectToPeer(addr string) {
	ui.eth.ConnectToPeer(addr)
}

func (ui *UiLib) AssetPath(p string) string {
	return path.Join(ui.assetPath, p)
}

func DefaultAssetPath() string {
	var base string
	// If the current working directory is the go-ethereum dir
	// assume a debug build and use the source directory as
	// asset directory.
	pwd, _ := os.Getwd()
	if pwd == path.Join(os.Getenv("GOPATH"), "src", "github.com", "ethereum", "go-ethereum", "ethereal") {
		base = path.Join(pwd, "assets")
	} else {
		switch runtime.GOOS {
		case "darwin":
			// Get Binary Directory
			exedir, _ := osext.ExecutableFolder()
			base = filepath.Join(exedir, "../Resources")
		case "linux":
			base = "/usr/share/ethereal"
		case "window":
			fallthrough
		default:
			base = "."
		}
	}

	return base
}

type memAddr struct {
	Num   string
	Value string
}

func (ui *UiLib) DebugTx(recipient, valueStr, gasStr, gasPriceStr, data string) (string, error) {
	state := ui.eth.BlockChain().CurrentBlock.State()

	asm, err := mutan.Compile(strings.NewReader(data), false)
	if err != nil {
		fmt.Println(err)
	}

	callerScript := ethutil.Assemble(asm...)
	dis := ethchain.Disassemble(callerScript)
	ui.win.Root().Call("clearAsm")
	for _, str := range dis {
		ui.win.Root().Call("setAsm", str)
	}
	callerTx := ethchain.NewContractCreationTx(ethutil.Big(valueStr), ethutil.Big(gasPriceStr), callerScript)

	// Contract addr as test address
	keyPair := ethutil.Config.Db.GetKeys()[0]
	account := ui.eth.StateManager().GetAddrState(keyPair.Address()).Account
	c := ethchain.MakeContract(callerTx, state)
	callerClosure := ethchain.NewClosure(account, c, c.Script(), state, ethutil.Big(gasStr), new(big.Int))

	block := ui.eth.BlockChain().CurrentBlock
	vm := ethchain.NewVm(state, ethchain.RuntimeVars{
		Origin:      account.Address(),
		BlockNumber: block.BlockInfo().Number,
		PrevHash:    block.PrevHash,
		Coinbase:    block.Coinbase,
		Time:        block.Time,
		Diff:        block.Difficulty,
		TxData:      nil,
	})
	callerClosure.Call(vm, nil, func(op ethchain.OpCode, mem *ethchain.Memory, stack *ethchain.Stack) {
		ui.win.Root().Call("clearMem")
		ui.win.Root().Call("clearStack")

		addr := 0
		for i := 0; i+32 <= mem.Len(); i += 32 {
			ui.win.Root().Call("setMem", memAddr{fmt.Sprintf("%03d", addr), fmt.Sprintf("% x", mem.Data()[i:i+32])})
			addr++
		}

		for _, val := range stack.Data() {
			ui.win.Root().Call("setStack", val.String())
		}
	})
	state.Reset()

	return "", nil
}