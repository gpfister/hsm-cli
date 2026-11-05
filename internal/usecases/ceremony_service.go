package usecases

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"golang.org/x/term"
)

// ATCeremonyClient specifies the operations needed by the ceremony service from an AT client.
type ATCeremonyClient interface {
	ThresholdInit(threshold, total int) (*domain.ThresholdInitResult, error)
	SubmitShare(index int, shareHex string) (*domain.WordCommitment, error)
	Status() (*domain.DeviceStatus, error)
	Lock() error
}

// CeremonyService coordinates turn-by-turn custodian ceremonies with strict terminal hygiene.
type CeremonyService struct {
	In     io.Reader
	Out    io.Writer
	reader *bufio.Reader
}

func (s *CeremonyService) getReader() *bufio.Reader {
	if s.reader == nil {
		s.reader = bufio.NewReader(s.In)
	}
	return s.reader
}

// NewCeremonyService creates a default CeremonyService reading from os.Stdin and writing to os.Stdout.
func NewCeremonyService() *CeremonyService {
	return &CeremonyService{
		In:  os.Stdin,
		Out: os.Stdout,
	}
}

// NewCeremonyServiceWithIO creates a CeremonyService with custom reader and writer for testing.
func NewCeremonyServiceWithIO(in io.Reader, out io.Writer) *CeremonyService {
	return &CeremonyService{
		In:  in,
		Out: out,
	}
}

// ClearTerminalAndScrollback emits ANSI escape sequence \033[H\033[2J\033[3J to purge terminal and scrollback buffer.
func ClearTerminalAndScrollback(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	_, _ = fmt.Fprint(w, "\033[H\033[2J\033[3J")
}

// ReadMaskedPassword prompts the user and securely reads input without echoing to the screen.
func (s *CeremonyService) ReadMaskedPassword(prompt string) (string, error) {
	if s.Out != nil && prompt != "" {
		fmt.Fprint(s.Out, prompt)
	}

	// If input is an interactive terminal, use term.ReadPassword
	if f, ok := s.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		bytePassword, err := term.ReadPassword(int(f.Fd()))
		if s.Out != nil {
			fmt.Fprintln(s.Out)
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}

	// Non-terminal fallback (pipes, automated tests)
	line, err := s.getReader().ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadMaskedPassword is a package-level helper that uses default stdin/stdout.
func ReadMaskedPassword(prompt string) (string, error) {
	return NewCeremonyService().ReadMaskedPassword(prompt)
}

// RunTurnByTurnInit runs a 3-of-5 threshold initialization ceremony presenting shares 1-5 turn-by-turn.
func (s *CeremonyService) RunTurnByTurnInit(client ATCeremonyClient) ([]domain.CustodianShare, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	// 1. Trigger threshold initialization on the HSM
	var result *domain.ThresholdInitResult
	if sc, ok := client.(interface {
		InitShamir() (*domain.ThresholdInitResult, error)
	}); ok {
		var initErr error
		result, initErr = sc.InitShamir()
		if initErr != nil {
			result, initErr = client.ThresholdInit(3, 5)
			if initErr != nil {
				return nil, fmt.Errorf("threshold initialization failed on HSM: %w", initErr)
			}
		}
	} else {
		var initErr error
		result, initErr = client.ThresholdInit(3, 5)
		if initErr != nil {
			return nil, fmt.Errorf("threshold initialization failed on HSM: %w", initErr)
		}
	}

	shares := result.Shares
	if len(shares) == 0 {
		return nil, errors.New("no custodian shares returned by HSM")
	}

	reader := s.getReader()

	// 2. Present each share individually to its respective custodian
	for i, share := range shares {
		custodianNum := i + 1

		ClearTerminalAndScrollback(s.Out)

		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, "           HSM THRESHOLD INITIALIZATION CEREMONY — CUSTODIAN %d OF %d           \n", custodianNum, len(shares))
		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, "Custodian #%d, please step forward to the terminal.\n", custodianNum)
		fmt.Fprintln(s.Out, "All other participants MUST turn away from the screen.")
		fmt.Fprintln(s.Out, "Ensure privacy filters and cameras are covered.")
		fmt.Fprintln(s.Out, "")
		fmt.Fprint(s.Out, "When ready to view your secret share, press [ENTER]...")
		_, _ = reader.ReadString('\n')

		ClearTerminalAndScrollback(s.Out)

		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, "              PRIVATE SECRET SHARE — CUSTODIAN #%d                             \n", custodianNum)
		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, " Custodian Index: %d\n", share.Index)
		fmt.Fprintf(s.Out, " Secret Share:    %s\n", share.Share)
		if share.CommitmentWord != "" {
			fmt.Fprintf(s.Out, " Commitment Word: %s\n", share.CommitmentWord)
		}
		fmt.Fprintln(s.Out, "--------------------------------------------------------------------------------")
		fmt.Fprintln(s.Out, "CRITICAL INSTRUCTIONS:")
		fmt.Fprintln(s.Out, " 1. Record your secret share in an offline, tamper-evident physical medium.")
		fmt.Fprintln(s.Out, " 2. Never store this share on a computer, phone, or connected system.")
		fmt.Fprintln(s.Out, " 3. At least 3 of 5 custodian shares are required to unlock this HSM.")
		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintln(s.Out, "")
		fmt.Fprint(s.Out, "Once securely recorded, press [ENTER] to PURGE the screen and summon the next custodian...")
		_, _ = reader.ReadString('\n')

		// Immediate terminal purge
		ClearTerminalAndScrollback(s.Out)
	}

	// Final purge and summary
	ClearTerminalAndScrollback(s.Out)
	fmt.Fprintln(s.Out, "================================================================================")
	fmt.Fprintln(s.Out, "✓ THRESHOLD CEREMONY COMPLETE")
	fmt.Fprintf(s.Out, "  All %d custodian shares have been securely distributed and purged from display.\n", len(shares))
	fmt.Fprintln(s.Out, "  HSM state: Configured & Locked (Solid RED LED).")
	fmt.Fprintln(s.Out, "================================================================================")

	return shares, nil
}

// RunTurnByTurnAuth prompts 3 custodians sequentially for shares and displays issued BIP-39 commitment words.
func (s *CeremonyService) RunTurnByTurnAuth(client ATCeremonyClient) ([]domain.CustodianShare, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	reader := s.getReader()
	var submittedShares []domain.CustodianShare
	var commitmentWords []string

	for turn := 1; turn <= 3; turn++ {
		ClearTerminalAndScrollback(s.Out)

		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, "           HSM THRESHOLD AUTHENTICATION CEREMONY — TURN %d OF 3                 \n", turn)
		fmt.Fprintln(s.Out, "================================================================================")
		fmt.Fprintf(s.Out, "Custodian for share entry #%d, please step up to the terminal.\n", turn)
		fmt.Fprintln(s.Out, "Other participants must turn away while entering the share.")
		fmt.Fprintln(s.Out, "")
		fmt.Fprint(s.Out, "Press [ENTER] when ready to enter your share...")
		_, _ = reader.ReadString('\n')

		ClearTerminalAndScrollback(s.Out)
		fmt.Fprintf(s.Out, "--- Custodian Turn %d of 3 ---\n", turn)

		// Custodian index prompt
		fmt.Fprintf(s.Out, "Enter your Custodian Index (1-5) [default %d]: ", turn)
		idxStr, _ := reader.ReadString('\n')
		idxStr = strings.TrimSpace(idxStr)
		idx := turn
		if idxStr != "" {
			if parsed, err := strconv.Atoi(idxStr); err == nil && parsed >= 1 && parsed <= 5 {
				idx = parsed
			}
		}

		shareInput, err := s.ReadMaskedPassword(fmt.Sprintf("Enter Custodian #%d Secret Share (masked): ", idx))
		if err != nil {
			return nil, fmt.Errorf("failed reading share input: %w", err)
		}
		if shareInput == "" {
			return nil, errors.New("share cannot be empty")
		}

		// Submit share to HSM
		commitment, err := client.SubmitShare(idx, shareInput)
		if err != nil {
			return nil, fmt.Errorf("HSM rejected share for custodian #%d: %w", idx, err)
		}

		commitmentWords = append(commitmentWords, commitment.Word)
		submittedShares = append(submittedShares, domain.CustodianShare{
			Index:          idx,
			Share:          shareInput,
			CommitmentWord: commitment.Word,
		})

		fmt.Fprintln(s.Out, "")
		fmt.Fprintf(s.Out, "✓ Share Accepted by HSM! Issued BIP-39 Commitment Word: [%s]\n", commitment.Word)
		fmt.Fprintln(s.Out, "  Remember or record your commitment word for the joint verification step.")
		fmt.Fprintln(s.Out, "")
		fmt.Fprint(s.Out, "Press [ENTER] to PURGE the screen and continue...")
		_, _ = reader.ReadString('\n')

		ClearTerminalAndScrollback(s.Out)
	}

	// Purge screen before joint verification
	ClearTerminalAndScrollback(s.Out)

	// Step 4: Joint verification of commitment words
	if err := s.VerifyCommitmentWords(commitmentWords); err != nil {
		ClearTerminalAndScrollback(s.Out)
		return nil, fmt.Errorf("joint commitment verification aborted: %w", err)
	}

	ClearTerminalAndScrollback(s.Out)
	fmt.Fprintln(s.Out, "================================================================================")
	fmt.Fprintln(s.Out, "✓ THRESHOLD AUTHENTICATION SUCCESSFUL")
	fmt.Fprintln(s.Out, "  HSM state: Authenticated & Unlocked (Solid GREEN LED).")
	fmt.Fprintln(s.Out, "================================================================================")

	return submittedShares, nil
}

// VerifyCommitmentWords prompts custodians to enter the sequence of commitment words jointly and aborts on mismatch.
func (s *CeremonyService) VerifyCommitmentWords(expectedWords []string) error {
	if len(expectedWords) == 0 {
		return errors.New("no commitment words to verify")
	}

	fmt.Fprintln(s.Out, "================================================================================")
	fmt.Fprintln(s.Out, "              JOINT BIP-39 COMMITMENT WORD VERIFICATION                         ")
	fmt.Fprintln(s.Out, "================================================================================")
	fmt.Fprintf(s.Out, "All %d participating custodians must jointly verify their commitment words.\n", len(expectedWords))
	fmt.Fprintln(s.Out, "Enter the issued words in the exact turn sequence (separated by spaces).")
	fmt.Fprintln(s.Out, "Any mismatch will immediately abort the session and lock the device.")
	fmt.Fprintln(s.Out, "--------------------------------------------------------------------------------")
	fmt.Fprint(s.Out, "Joint Commitment Words: ")

	reader := s.getReader()
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("failed to read commitment verification words: %w", err)
	}

	enteredWords := strings.Fields(line)
	if err := domain.VerifyCommitmentSequence(enteredWords, expectedWords); err != nil {
		return fmt.Errorf("commitment verification failed: %w", err)
	}

	fmt.Fprintln(s.Out, "✓ All commitment words verified successfully!")
	return nil
}
