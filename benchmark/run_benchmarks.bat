@echo off
echo Running GojiDB Benchmark Tests...
echo.

REM 设置环境变量
set GO111MODULE=on

REM 运行基础基准测试
echo === Basic Benchmark Tests ===
go test -bench=BenchmarkPut -benchmem -count=3 ./benchmark
echo.

echo === Basic Get Benchmark ===
go test -bench=BenchmarkGet -benchmem -count=3 ./benchmark
echo.

echo === Batch Operations Benchmark ===
go test -bench=BenchmarkBatch -benchmem -count=3 ./benchmark
echo.

REM 运行并发基准测试
echo === Concurrent Benchmark Tests ===
go test -bench=BenchmarkConcurrent -benchmem -count=3 ./benchmark
echo.

REM 运行缓存基准测试
echo === Cache Benchmark Tests ===
go test -bench=BenchmarkCache -benchmem -count=3 ./benchmark
echo.

REM 运行大小基准测试
echo === Size Benchmark Tests ===
go test -bench=BenchmarkSmall -benchmem -count=3 ./benchmark
go test -bench=BenchmarkMedium -benchmem -count=3 ./benchmark
go test -bench=BenchmarkLarge -benchmem -count=3 ./benchmark
echo.

REM 运行所有基准测试并生成报告
echo === Running All Benchmarks ===
go test -bench=. -benchmem -count=5 ./benchmark > benchmark_results.txt
echo Results saved to benchmark_results.txt
echo.

echo Benchmark tests completed!