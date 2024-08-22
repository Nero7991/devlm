package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(code string, timeout time.Duration) (string, error) {
	args := m.Called(code, timeout)
	return args.String(0), args.Error(1)
}

func TestSandbox_Execute(t *testing.T) {
	mockExec := new(MockExecutor)
	sandbox := NewSandbox(mockExec)

	testCases := []struct {
		name     string
		code     string
		timeout  time.Duration
		expected string
		err      error
	}{
		{
			name:     "Simple execution",
			code:     "print('Hello, World!')",
			timeout:  5 * time.Second,
			expected: "Hello, World!",
			err:      nil,
		},
		{
			name:     "Execution with timeout",
			code:     "import time; time.sleep(10); print('Done')",
			timeout:  1 * time.Second,
			expected: "",
			err:      ErrExecutionTimeout,
		},
		{
			name:     "Execution with syntax error",
			code:     "print('Unclosed string)",
			timeout:  5 * time.Second,
			expected: "",
			err:      ErrExecutionFailed,
		},
		{
			name:     "Execution with runtime error",
			code:     "1/0",
			timeout:  5 * time.Second,
			expected: "",
			err:      ErrExecutionFailed,
		},
		{
			name:     "Execution with large output",
			code:     "print('a' * 1000000)",
			timeout:  5 * time.Second,
			expected: "a" * 1000000,
			err:      nil,
		},
		{
			name:     "Execution with Unicode characters",
			code:     "print('こんにちは世界')",
			timeout:  5 * time.Second,
			expected: "こんにちは世界",
			err:      nil,
		},
		{
			name:     "Execution with complex calculation",
			code:     "print(sum(x**2 for x in range(1000)))",
			timeout:  5 * time.Second,
			expected: "332833500",
			err:      nil,
		},
		{
			name:     "Execution with recursive function",
			code:     "def fib(n): return n if n < 2 else fib(n-1) + fib(n-2)\nprint(fib(20))",
			timeout:  5 * time.Second,
			expected: "6765",
			err:      nil,
		},
		{
			name:     "Execution with infinite loop",
			code:     "while True: pass",
			timeout:  1 * time.Second,
			expected: "",
			err:      ErrExecutionTimeout,
		},
		{
			name:     "Execution with multiple functions",
			code:     "def add(a, b): return a + b\ndef multiply(a, b): return a * b\nprint(add(multiply(2, 3), 4))",
			timeout:  5 * time.Second,
			expected: "10",
			err:      nil,
		},
		{
			name:     "Execution with imported module",
			code:     "import math\nprint(math.pi)",
			timeout:  5 * time.Second,
			expected: "3.141592653589793",
			err:      nil,
		},
		{
			name:     "Execution with list comprehension",
			code:     "print([x for x in range(10) if x % 2 == 0])",
			timeout:  5 * time.Second,
			expected: "[0, 2, 4, 6, 8]",
			err:      nil,
		},
		{
			name:     "Execution with generator expression",
			code:     "print(sum(x for x in range(1000) if x % 3 == 0))",
			timeout:  5 * time.Second,
			expected: "166833",
			err:      nil,
		},
		{
			name:     "Execution with complex string operations",
			code:     "print(''.join(chr((ord(c) + 1) % 256) for c in 'Hello, World!'))",
			timeout:  5 * time.Second,
			expected: "Ifmmp-!Xpsme\"",
			err:      nil,
		},
		{
			name:     "Execution with nested functions and closures",
			code:     "def outer(x):\n    def inner(y):\n        return x + y\n    return inner\nf = outer(10)\nprint(f(5))",
			timeout:  5 * time.Second,
			expected: "15",
			err:      nil,
		},
		{
			name:     "Execution with class definition and method calls",
			code:     "class TestClass:\n    def __init__(self, value):\n        self.value = value\n    def double(self):\n        return self.value * 2\nobj = TestClass(21)\nprint(obj.double())",
			timeout:  5 * time.Second,
			expected: "42",
			err:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockExec.On("Execute", tc.code, tc.timeout).Return(tc.expected, tc.err)

			result, err := sandbox.Execute(tc.code, tc.timeout)

			assert.Equal(t, tc.expected, result)
			assert.Equal(t, tc.err, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestSandbox_ExecuteWithResourceLimits(t *testing.T) {
	mockExec := new(MockExecutor)
	sandbox := NewSandbox(mockExec)

	testCases := []struct {
		name          string
		code          string
		timeout       time.Duration
		memoryLimit   int64
		cpuLimit      float64
		expected      string
		err           error
		expectedLimits ResourceLimits
	}{
		{
			name:          "Execution with resource limits",
			code:          "print('Resource limited')",
			timeout:       5 * time.Second,
			memoryLimit:   100 * 1024 * 1024,
			cpuLimit:      1.0,
			expected:      "Resource limited",
			err:           nil,
			expectedLimits: ResourceLimits{
				MemoryMB: 100,
				CPUCores: 1.0,
			},
		},
		{
			name:          "Execution exceeding memory limit",
			code:          "a = ' ' * (1024 * 1024 * 200)",
			timeout:       5 * time.Second,
			memoryLimit:   50 * 1024 * 1024,
			cpuLimit:      1.0,
			expected:      "",
			err:           ErrMemoryLimitExceeded,
			expectedLimits: ResourceLimits{
				MemoryMB: 50,
				CPUCores: 1.0,
			},
		},
		{
			name:          "Execution exceeding CPU limit",
			code:          "import time; [x**2 for x in range(10**7)]",
			timeout:       5 * time.Second,
			memoryLimit:   100 * 1024 * 1024,
			cpuLimit:      0.1,
			expected:      "",
			err:           ErrCPULimitExceeded,
			expectedLimits: ResourceLimits{
				MemoryMB: 100,
				CPUCores: 0.1,
			},
		},
		{
			name:          "Execution with minimal resources",
			code:          "print('Minimal resources')",
			timeout:       1 * time.Second,
			memoryLimit:   1 * 1024 * 1024,
			cpuLimit:      0.1,
			expected:      "Minimal resources",
			err:           nil,
			expectedLimits: ResourceLimits{
				MemoryMB: 1,
				CPUCores: 0.1,
			},
		},
		{
			name:          "Execution with maximum resources",
			code:          "print('Maximum resources')",
			timeout:       30 * time.Second,
			memoryLimit:   1024 * 1024 * 1024,
			cpuLimit:      8.0,
			expected:      "Maximum resources",
			err:           nil,
			expectedLimits: ResourceLimits{
				MemoryMB: 1024,
				CPUCores: 8.0,
			},
		},
		{
			name:          "Execution with high CPU usage",
			code:          "import math; [math.factorial(1000) for _ in range(1000000)]",
			timeout:       10 * time.Second,
			memoryLimit:   512 * 1024 * 1024,
			cpuLimit:      2.0,
			expected:      "",
			err:           ErrCPULimitExceeded,
			expectedLimits: ResourceLimits{
				MemoryMB: 512,
				CPUCores: 2.0,
			},
		},
		{
			name:          "Execution with high memory usage",
			code:          "a = [i for i in range(10**8)]",
			timeout:       10 * time.Second,
			memoryLimit:   256 * 1024 * 1024,
			cpuLimit:      4.0,
			expected:      "",
			err:           ErrMemoryLimitExceeded,
			expectedLimits: ResourceLimits{
				MemoryMB: 256,
				CPUCores: 4.0,
			},
		},
		{
			name:          "Execution with borderline resource usage",
			code:          "a = [i for i in range(10**6)]; print(len(a))",
			timeout:       5 * time.Second,
			memoryLimit:   100 * 1024 * 1024,
			cpuLimit:      1.0,
			expected:      "1000000",
			err:           nil,
			expectedLimits: ResourceLimits{
				MemoryMB: 100,
				CPUCores: 1.0,
			},
		},
		{
			name:          "Execution with multiple resource-intensive operations",
			code:          "import math; a = [math.factorial(100) for _ in range(10000)]; print(len(a))",
			timeout:       10 * time.Second,
			memoryLimit:   512 * 1024 * 1024,
			cpuLimit:      4.0,
			expected:      "10000",
			err:           nil,
			expectedLimits: ResourceLimits{
				MemoryMB: 512,
				CPUCores: 4.0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockExec.On("Execute", tc.code, tc.timeout).Return(tc.expected, tc.err)

			result, err := sandbox.ExecuteWithResourceLimits(tc.code, tc.timeout, tc.memoryLimit, tc.cpuLimit)

			assert.Equal(t, tc.expected, result)
			assert.Equal(t, tc.err, err)
			assert.Equal(t, tc.expectedLimits, sandbox.resourceLimits)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestSandbox_SetResourceLimits(t *testing.T) {
	mockExec := new(MockExecutor)
	sandbox := NewSandbox(mockExec)

	testCases := []struct {
		name          string
		limits        ResourceLimits
		expectedError error
	}{
		{
			name: "Valid limits",
			limits: ResourceLimits{
				MemoryMB: 200,
				CPUCores: 2.0,
			},
			expectedError: nil,
		},
		{
			name: "Invalid memory limit",
			limits: ResourceLimits{
				MemoryMB: -100,
				CPUCores: 1.0,
			},
			expectedError: ErrInvalidResourceLimits,
		},
		{
			name: "Invalid CPU limit",
			limits: ResourceLimits{
				MemoryMB: 100,
				CPUCores: -1.5,
			},
			expectedError: ErrInvalidResourceLimits,
		},
		{
			name: "Zero memory limit",
			limits: ResourceLimits{
				MemoryMB: 0,
				CPUCores: 1.0,
			},
			expectedError: ErrInvalidResourceLimits,
		},
		{
			name: "Zero CPU limit",
			limits: ResourceLimits{
				MemoryMB: 100,
				CPUCores: 0,
			},
			expectedError: ErrInvalidResourceLimits,
		},
		{
			name: "Very high limits",
			limits: ResourceLimits{
				MemoryMB: 1024 * 1024,
				CPUCores: 128,
			},
			expectedError: nil,
		},
		{
			name: "Fractional CPU cores",
			limits: ResourceLimits{
				MemoryMB: 512,
				CPUCores: 0.5,
			},
			expectedError: nil,
		},
		{
			name: "Extremely low memory",
			limits: ResourceLimits{
				MemoryMB: 1,
				CPUCores: 0.1,
			},
			expectedError: nil,
		},
		{
			name: "Maximum allowed limits",
			limits: ResourceLimits{
				MemoryMB: 1024 * 1024,
				CPUCores: 256,
			},
			expectedError: nil,
		},
		{
			name: "Exceeding maximum memory limit",
			limits: ResourceLimits{
				MemoryMB: 1024*1024 + 1,
				CPUCores: 1.0,
			},
			expectedError: ErrInvalidResourceLimits,
		},
		{
			name: "Exceeding maximum CPU limit",
			limits: ResourceLimits{
				MemoryMB: 1024,
				CPUCores: 256.1,
			},
			expectedError: ErrInvalidResourceLimits,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := sandbox.SetResourceLimits(tc.limits)

			if tc.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.limits, sandbox.resourceLimits)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedError, err)
			}
		})
	}
}

func TestSandbox_ExecuteWithIsolation(t *testing.T) {
	mockExec := new(MockExecutor)
	sandbox := NewSandbox(mockEx