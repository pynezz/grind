GO ?= go

.PHONY: build test vet bench dashboard fmt clean

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# Difficulty-scaling benchmark from PLAN.md's calibration table.
bench:
	$(GO) test ./puzzle/ -bench BenchmarkSolve -run '^$$' -benchtime=3x

# Regenerates tools/dashboard/dist.html from a fresh test+benchmark run
# (see tools/dashboard/collect.py). Republishing that file to the live
# "Grind" artifact is a separate step, done via the Artifact tool.
dashboard:
	python3 tools/dashboard/collect.py
	python3 tools/dashboard/render.py

fmt:
	gofmt -l .

clean:
	rm -f tools/dashboard/dist.html
