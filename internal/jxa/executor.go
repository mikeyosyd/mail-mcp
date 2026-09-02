package jxa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/dastrobu/mail-mcp/internal/log"
)

// Result represents the result of a JXA script execution
type Result struct {
	Success   bool           `json:"success"`
	Data      map[string]any `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
	ErrorCode string         `json:"errorCode,omitempty"`
}

// Error codes returned by JXA scripts
const (
	ErrorCodeMailAppNotRunning    = "MAIL_APP_NOT_RUNNING"
	ErrorCodeMailAppNoPermissions = "MAIL_APP_NO_PERMISSIONS"
)

// execLock serialises osascript runs across concurrent callers.
//
// Mail.app handles AppleEvents one at a time, so two osascript processes
// started together do not run in parallel: the second one queues inside
// Mail.app behind the first. If that queue is long enough, the MCP client's
// tool timeout fires for the second call, its context is cancelled and its
// osascript process is killed mid-flight. The failure surfaces as
// "The operation timed out" and looks like a slow mailbox when it is not.
//
// Queueing here instead keeps at most one osascript process alive at a time
// and lets a waiting caller give up cleanly through its context. A buffered
// channel of size 1 is used as the mutex, rather than sync.Mutex, so that
// acquisition can be selected against ctx.Done().
var execLock = make(chan struct{}, 1)

// runOSAScript runs osascript with the given arguments and returns its
// combined stdout and stderr. It is a variable so tests can substitute a fake.
var runOSAScript = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "osascript", args...).CombinedOutput()
}

// acquire takes execLock, waiting for any in-flight osascript run to finish
// first. If ctx is cancelled while waiting, it returns an error wrapping
// ctx.Err() instead and the lock is not held.
func acquire(ctx context.Context) error {
	// Fast path: nothing is running.
	select {
	case execLock <- struct{}{}:
		return nil
	default:
	}

	// Slow path: wait for the running script, unless ctx is cancelled first.
	select {
	case execLock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("cancelled while waiting for an earlier JXA script to finish: %w", ctx.Err())
	}
}

// release gives up execLock. It must only be called after a successful acquire.
func release() {
	<-execLock
}

// Execute runs a JXA script with the given arguments and returns the parsed result.
//
// Executions are serialised: if another script is still running, this call
// waits for it to finish before starting osascript. The wait is abandoned
// with an error wrapping ctx.Err() if ctx is cancelled first.
func Execute(ctx context.Context, script string, args ...string) (any, error) {
	if err := acquire(ctx); err != nil {
		return nil, fmt.Errorf("osascript not started: %w\nArguments: %v", err, args)
	}
	defer release()

	// Build osascript command
	cmdArgs := []string{"-l", "JavaScript", "-e", script}
	cmdArgs = append(cmdArgs, args...)

	output, err := runOSAScript(ctx, cmdArgs...)
	if err != nil {
		// Provide more context about the failure
		if len(output) > 0 {
			return nil, fmt.Errorf("osascript execution failed: %w\nOutput: %s\nArguments: %v", err, string(output), args)
		}
		return nil, fmt.Errorf("osascript execution failed: %w\nArguments: %v", err, args)
	}

	// Check if output is empty
	if len(output) == 0 {
		return nil, fmt.Errorf("osascript returned empty output (expected JSON)\nArguments: %v", args)
	}

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse osascript JSON output: %w\nRaw output: %s\nArguments: %v", err, string(output), args)
	}

	// Check for script-level errors
	success, hasSuccess := result["success"].(bool)
	if !hasSuccess {
		return nil, fmt.Errorf("script output missing 'success' field or invalid type\nOutput: %s\nArguments: %v", string(output), args)
	}

	if !success {
		errMsg := "unknown error (script returned success=false with no error message)"
		if errVal, ok := result["error"].(string); ok && errVal != "" {
			errMsg = errVal
		}

		// Check for specific error codes
		if errorCode, ok := result["errorCode"].(string); ok {
			switch errorCode {
			case ErrorCodeMailAppNotRunning:
				return nil, fmt.Errorf("Mail.app is not running. Please start Mail.app and try again")
			case ErrorCodeMailAppNoPermissions:
				return nil, fmt.Errorf("Mail.app automation permission denied. Please grant permission to %q in System Settings > Privacy & Security > Automation", os.Args[0])
			}
		}

		// Include logs if available for better debugging
		if logs, ok := result["logs"].(string); ok && logs != "" {
			return nil, fmt.Errorf("JXA script error: %s\nLogs:\n%s\nArguments: %v", errMsg, logs, args)
		}

		return nil, fmt.Errorf("JXA script error: %s\nArguments: %v", errMsg, args)
	}

	// Extract and return data field
	data, ok := result["data"]
	if !ok {
		return nil, fmt.Errorf("script output missing 'data' field\nOutput: %s\nArguments: %v", string(output), args)
	}

	// Log JXA script logs using logger from context
	logger := log.FromContext(ctx)
	if logs, ok := result["logs"].(string); ok && logs != "" {
		logger.Printf("[DEBUG] JXA Script Logs:\n%s\n", logs)
	}

	return data, nil
}
