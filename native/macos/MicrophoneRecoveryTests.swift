import Foundation

private enum TestFailure: Error { case restart }

private final class FakeMicrophone: RecoverableMicrophone {
    var isRunning = false
    var formatUnchanged = true
    var starts = 0
    var stops = 0
    var failNext = false
    func restart() throws {
        starts += 1
        if failNext { failNext = false; throw TestFailure.restart }
        isRunning = true
        formatUnchanged = true
    }
    func stop() { stops += 1; isRunning = false }
}

private final class Harness {
    let engine = FakeMicrophone()
    var scheduled: [() -> Void] = []
    var states: [String] = []
    var errors: [String] = []
    var time: TimeInterval = 0
    lazy var recovery = MicrophoneRecovery(
        engine: engine,
        schedule: { [unowned self] in self.scheduled.append($0) },
        now: { [unowned self] in self.time },
        onState: { [unowned self] in self.states.append($0) },
        onError: { [unowned self] in self.errors.append($0) })
    func runPending() {
        let work = scheduled
        scheduled.removeAll()
        for task in work { task() }
    }
    func loseDevice() {
        engine.isRunning = false
        engine.formatUnchanged = false
        recovery.configurationChanged()
    }
}

@main
struct RecoveryTests {
    static func check(_ value: @autoclosure () -> Bool, _ message: String) {
        if !value() {
            FileHandle.standardError.write(Data(("失败：" + message + "\n").utf8))
            exit(1)
        }
    }

    static func main() throws {
        let h = Harness()
        try h.recovery.start()
        h.loseDevice()
        h.runPending()
        check(h.engine.starts == 2 && h.engine.isRunning, "蓝牙重配置停止引擎后，应重建输入并恢复采集")
        check(h.states.suffix(2) == ["microphone-restarting", "microphone-restarted"], "恢复过程必须输出固定状态诊断")

        for _ in 0..<20 { h.recovery.configurationChanged() }
        h.runPending()
        check(h.engine.starts == 2, "运行正常时的重复通知不能重建 tap")

        h.loseDevice()
        for _ in 0..<20 { h.recovery.configurationChanged() }
        check(h.scheduled.count == 1, "密集配置通知应合并为一次恢复")
        h.runPending()
        check(h.engine.starts == 3, "合并后只恢复一次")

        h.loseDevice()
        h.recovery.stop()
        h.runPending()
        check(h.engine.starts == 3 && !h.engine.isRunning, "停止后不能执行已排队的恢复")
        h.recovery.configurationChanged()
        check(h.scheduled.isEmpty, "停止后新通知不能继续调度")

        let superseded = Harness()
        try superseded.recovery.start()
        superseded.loseDevice()
        superseded.recovery.stop()
        try superseded.recovery.start()
        superseded.runPending()
        check(superseded.engine.starts == 2, "前一会话的恢复任务不能重启新会话的引擎")

        let formatChange = Harness()
        try formatChange.recovery.start()
        formatChange.engine.formatUnchanged = false
        formatChange.recovery.configurationChanged()
        formatChange.runPending()
        check(formatChange.engine.starts == 2 && formatChange.engine.formatUnchanged, "引擎仍运行但输入格式变化时也应重建转换器和 tap")

        let selfRecovered = Harness()
        try selfRecovered.recovery.start()
        selfRecovered.loseDevice()
        selfRecovered.engine.isRunning = true
        selfRecovered.engine.formatUnchanged = true
        selfRecovered.runPending()
        check(selfRecovered.engine.starts == 1, "等待期间系统自行恢复后不应再重启")

        try h.recovery.start()
        h.loseDevice()
        h.engine.failNext = true
        h.runPending()
        check(h.errors.count == 1 && !h.engine.isRunning, "恢复失败必须停止并明确报错")
        h.recovery.configurationChanged()
        check(h.scheduled.isEmpty, "失败后不能无限重试")

        let storm = Harness()
        try storm.recovery.start()
        for _ in 0..<4 { storm.loseDevice(); storm.runPending() }
        check(storm.engine.starts == 4 && storm.errors.count == 1, "30 秒内最多恢复三次，通知风暴不能无限重启")

        let spaced = Harness()
        try spaced.recovery.start()
        for _ in 0..<4 { spaced.time += 31; spaced.loseDevice(); spaced.runPending() }
        check(spaced.engine.starts == 5 && spaced.errors.isEmpty, "分散的设备变化不应永久耗尽恢复次数")

        print("通过：设备重配置恢复、重复通知、通知合并、停止取消、旧任务失效、运行中格式变化、系统自行恢复、失败上报与恢复频率限制")
    }
}
