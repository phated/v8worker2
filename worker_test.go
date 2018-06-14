/*
Copyright 2018 Ryan Dahl <ry@tinyclouds.org>. All rights reserved.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to
deal in the Software without restriction, including without limitation the
rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
IN THE SOFTWARE.
*/
package v8worker2

import (
	"log"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	println(Version())
}

func TestSetFlags(t *testing.T) {
	// One of the V8 flags to use as a test:
	//   --lazy (use lazy compilation)
	//      type: bool  default: true
	args := []string{"hello", "--lazy", "foobar"}
	modified := SetFlags(args)
	if len(modified) != 2 || modified[0] != "hello" || modified[1] != "foobar" {
		t.Fatalf("unexpected %v", modified)
	}
}

func TestPrint(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		t.Fatal("shouldn't recieve Message")
		return nil
	})
	err := worker.Load("code.js", `V8Worker2.print("ready");`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyntaxError(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		t.Fatal("shouldn't recieve Message")
		return nil
	})

	code := `V8Worker2.print(hello world");`
	err := worker.Load("codeWithSyntaxError.js", code)
	errorContains(t, err, "codeWithSyntaxError.js")
	errorContains(t, err, "hello")
}

func TestSendRecv(t *testing.T) {
	recvCount := 0
	worker := New(func(msg []byte) []byte {
		if len(msg) != 5 {
			t.Fatal("bad msg", msg)
		}
		recvCount++
		return nil
	})

	err := worker.Load("codeWithRecv.js", `
		V8Worker2.recv(function(msg) {
			V8Worker2.print("TestBasic recv byteLength", msg.byteLength);
			if (msg.byteLength !== 3) {
				throw Error("bad message");
			}
		});
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = worker.SendBytes([]byte("hii"))
	if err != nil {
		t.Fatal(err)
	}
	codeWithSend := `
		V8Worker2.send(new ArrayBuffer(5));
		V8Worker2.send(new ArrayBuffer(5));
	`
	err = worker.Load("codeWithSend.js", codeWithSend)
	if err != nil {
		t.Fatal(err)
	}

	if recvCount != 2 {
		t.Fatal("bad recvCount", recvCount)
	}
}

func TestSendWithReturnArrayBuffer(t *testing.T) {
	recvCount := 0
	worker := New(func(msg []byte) []byte {
		if len(msg) != 123 {
			t.Fatal("unexpected message")
		}
		recvCount++
		return []byte{1, 2, 3}
	})
	err := worker.Load("TestSendWithReturnArrayBuffer.js", `
		var ret = V8Worker2.send(new ArrayBuffer(123));
		if (!(ret instanceof ArrayBuffer)) throw Error("bad");
		if (ret.byteLength !== 3) throw Error("bad");
		ret = new Uint8Array(ret);
		if (ret[0] !== 1) throw Error("bad");
		if (ret[1] !== 2) throw Error("bad");
		if (ret[2] !== 3) throw Error("bad");
	`)
	if err != nil {
		t.Fatal(err)
	}
	if recvCount != 1 {
		t.Fatal("bad recvCount", recvCount)
	}
}

func TestThrowDuringLoad(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		return nil
	})
	err := worker.Load("TestThrowDuringLoad.js", `throw Error("bad");`)
	errorContains(t, err, "TestThrowDuringLoad.js")
	errorContains(t, err, "bad")
}

func TestThrowInRecvCallback(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		return nil
	})
	err := worker.Load("TestThrowInRecvCallback.js", `
		V8Worker2.recv(function(msg) {
			throw Error("bad");
		});
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = worker.SendBytes([]byte("abcd"))
	errorContains(t, err, "TestThrowInRecvCallback.js")
	errorContains(t, err, "bad")
}

func TestPrintUint8Array(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		return nil
	})
	err := worker.Load("buffer.js", `
		var uint8 = new Uint8Array(16);
		V8Worker2.print(uint8);
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipleWorkers(t *testing.T) {
	recvCount := 0
	worker1 := New(func(msg []byte) []byte {
		if len(msg) != 5 {
			t.Fatal("bad message")
		}
		recvCount++
		return nil
	})
	worker2 := New(func(msg []byte) []byte {
		if len(msg) != 3 {
			t.Fatal("bad message")
		}
		recvCount++
		return nil
	})

	err := worker1.Load("1.js", `V8Worker2.send(new ArrayBuffer(5))`)
	if err != nil {
		t.Fatal(err)
	}

	err = worker2.Load("2.js", `V8Worker2.send(new ArrayBuffer(3))`)
	if err != nil {
		t.Fatal(err)
	}

	if recvCount != 2 {
		t.Fatal("bad recvCount", recvCount)
	}
}

func TestRequestFromJS(t *testing.T) {
	var captured []byte
	worker := New(func(msg []byte) []byte {
		captured = msg
		return nil
	})
	code := ` V8Worker2.send(new ArrayBuffer(4)); `
	err := worker.Load("code.js", code)
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) != 4 {
		t.Fail()
	}
}

func TestModules(t *testing.T) {
	var worker *Worker
	worker = New(func(msg []byte) []byte {
		t.Fatal("shouldn't recieve Message")
		return nil
	})
	err2 := worker.LoadModule("code.js", `
		import { test } from "dependency.js";
		import { print } from "core:internal.js";

		print(test);

		if (typeof V8Worker2 != "undefined") {
			throw new Error('Global should not exist');
		}
	`, func(specifier string, referrer string) int {
		if specifier == "internal.js" {
			return 0
		}

		log.Println(specifier)
		if specifier != "dependency.js" {
			t.Fatal(`Expected "dependency.js" specifier`)
		}
		if referrer != "code.js" {
			t.Fatal(`Expected "code.js" referrer`)
		}
		err1 := worker.LoadModule("dependency.js", `
			export const test = "ready";
		`, func(_, _ string) int {
			t.Fatal(`Expected module resolver callback to not be called`)
			return 1
		})
		if err1 != nil {
			t.Fatal(err1)
		}
		return 0
	})
	if err2 != nil {
		t.Fatal(err2)
	}
}

func TestModulesMissingDependency(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		t.Fatal("shouldn't recieve Message")
		return nil
	})
	err := worker.LoadModule("code.js", `
		import { test } from "missing.js";
		throw new Error('Should not reach here');
	`, func(specifier string, referrer string) int {
		if specifier != "missing.js" {
			t.Fatal(`Expected "missing.js" specifier`)
		}
		return 1
	})
	errorContains(t, err, "missing.js")
}

func TestUnpriviledgedModule(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		t.Fatal("shouldn't recieve Message")
		return nil
	})
	err := worker.LoadModule("code.js", `
		import { print } from "core:internal.js";
		throw new Error("Should not reach here");
	`, func(specifier string, referrer string) int {
		if specifier != "core:internal.js" {
			t.Fatal(`Expected "core:internal.js" specifier`)
		}
		return 1
	})
	log.Println(err)
	errorContains(t, err, "core:internal.js")
}

// Test breaking script execution
func TestWorkerBreaking(t *testing.T) {
	worker := New(func(msg []byte) []byte {
		return nil
	})

	go func(w *Worker) {
		time.Sleep(time.Second)
		w.TerminateExecution()
	}(worker)

	worker.Load("forever.js", ` while (true) { ; } `)
}

func errorContains(t *testing.T, err error, substr string) {
	if err == nil {
		t.Fatal("Expected to get error")
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("Expected error to have '%s' in it.", substr)
	}
}
