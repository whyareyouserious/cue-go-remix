import Foundation

// 所有方法及 schedule 回调必须在同一串行队列执行。
protocol RecoverableMicrophone: AnyObject {
    var isRunning: Bool { get }
    var formatUnchanged: Bool { get }
    func restart() throws
    func stop()
}

final class MicrophoneRecovery {
    private let engine: RecoverableMicrophone
    private let schedule: (@escaping () -> Void) -> Void
    private let now: () -> TimeInterval
    private let onState: (String) -> Void
    private let onError: (String) -> Void
    private var active = false
    private var pending = false
    private var generation: UInt64 = 0
    private var attempts: [TimeInterval] = []

    init(engine: RecoverableMicrophone,
         schedule: @escaping (@escaping () -> Void) -> Void,
         now: @escaping () -> TimeInterval = { ProcessInfo.processInfo.systemUptime },
         onState: @escaping (String) -> Void,
         onError: @escaping (String) -> Void) {
        self.engine = engine
        self.schedule = schedule
        self.now = now
        self.onState = onState
        self.onError = onError
    }

    func start() throws {
        active = true
        pending = false
        generation &+= 1
        attempts.removeAll()
        do { try engine.restart() }
        catch { stop(); throw error }
    }

    func stop() {
        active = false
        pending = false
        generation &+= 1
        engine.stop()
    }

    func configurationChanged() {
        guard active else { return }
        onState("microphone-config-changed")
        guard !pending, !engine.isRunning || !engine.formatUnchanged else { return }
        pending = true
        generation &+= 1
        let ticket = generation
        schedule { [weak self] in self?.recover(ticket: ticket) }
    }

    private func recover(ticket: UInt64) {
        guard active, pending, generation == ticket else { return }
        pending = false
        // 系统可能已经自行恢复；此时不重复删除、安装 tap。
        guard !engine.isRunning || !engine.formatUnchanged else { return }
        let time = now()
        attempts.removeAll { time - $0 >= 30 }
        guard attempts.count < 3 else {
            stop()
            onError("麦克风设备反复变化，已停止采集；请确认输入设备后重新开始会话")
            return
        }
        attempts.append(time)
        onState("microphone-restarting")
        do {
            try engine.restart()
            onState("microphone-restarted")
        } catch {
            stop()
            onError("麦克风设备变化后恢复失败：" + error.localizedDescription)
        }
    }
}
