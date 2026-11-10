import Foundation
import AVFoundation
import ScreenCaptureKit
import CoreMedia
import Darwin

// stdout framing: kind (0 ready, 1 microphone, 2 system, 3 error, 4 state), UInt32
// little-endian byte count, payload. Audio payloads are mono PCM16 at 16 kHz.
final class Output: @unchecked Sendable {
    private let lock = NSLock()
    private var active = false

    func ready() {
        lock.lock()
        writeLocked(0, Data())
        active = true
        lock.unlock()
    }

    func audio(_ channel: UInt8, _ payload: Data) {
        lock.lock()
        defer { lock.unlock() }
        if active { writeLocked(channel, payload) }
    }

    func error(_ message: String) {
        lock.lock()
        defer { lock.unlock() }
        writeLocked(3, Data(message.utf8))
    }

    func diagnostic(_ state: String) {
        // 固定状态枚举，不包含声音内容或设备名称。
        guard ["microphone-config-changed", "microphone-restarting", "microphone-restarted"].contains(state) else { return }
        lock.lock()
        writeLocked(4, Data(state.utf8))
        lock.unlock()
    }

    func stop() {
        lock.lock(); active = false; lock.unlock()
    }

    private func writeLocked(_ kind: UInt8, _ payload: Data) {
        var length = UInt32(payload.count).littleEndian
        var frame = Data([kind])
        withUnsafeBytes(of: &length) { frame.append(contentsOf: $0) }
        frame.append(payload)
        do { try FileHandle.standardOutput.write(contentsOf: frame) }
        catch { Darwin.exit(0) } // Parent closed its pipe: do not keep recording.
    }
}

final class PCMConverter {
    private var converter: AVAudioConverter?
    private var inputFormat: AVAudioFormat?
    private let target = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16_000, channels: 1, interleaved: true)!

    func convert(_ input: AVAudioPCMBuffer) throws -> Data {
        if inputFormat != input.format {
            inputFormat = input.format
            converter = AVAudioConverter(from: input.format, to: target)
        }
        guard let converter else { throw CaptureError.message("无法转换音频设备的采样格式") }
        let capacity = AVAudioFrameCount(ceil(Double(input.frameLength) * 16_000 / input.format.sampleRate)) + 32
        guard let output = AVAudioPCMBuffer(pcmFormat: target, frameCapacity: capacity) else {
            throw CaptureError.message("无法建立音频转换缓冲区")
        }
        var supplied = false
        var error: NSError?
        let result = converter.convert(to: output, error: &error) { _, status in
            if supplied { status.pointee = .noDataNow; return nil }
            supplied = true
            status.pointee = .haveData
            return input
        }
        if result == .error { throw error ?? CaptureError.message("音频格式转换失败") as NSError }
        guard let samples = output.int16ChannelData else { return Data() }
        return Data(bytes: samples[0], count: Int(output.frameLength) * 2)
    }
}

enum CaptureError: LocalizedError {
    case message(String)
    var errorDescription: String? { if case .message(let text) = self { return text }; return nil }
}

// 生命周期由 AudioCapture.microphoneQueue 串行调用；每次 tap 单独持有
// converter，旧音频回调与重新配置不会并发改写同一个转换器。
final class MicrophoneInput: RecoverableMicrophone {
    private let engine = AVAudioEngine()
    private let output: Output
    private var tapped = false
    private var tapFormat: AVAudioFormat?

    init(output: Output) { self.output = output }
    var notificationObject: AVAudioEngine { engine }
    var isRunning: Bool { engine.isRunning }
    var formatUnchanged: Bool { tapFormat == engine.inputNode.outputFormat(forBus: 0) }

    func restart() throws {
        stop()
        engine.reset()
        let input = engine.inputNode
        let format = input.outputFormat(forBus: 0)
        guard format.channelCount > 0, format.sampleRate > 0 else {
            throw CaptureError.message("没有可用的麦克风，请连接输入设备后重试")
        }
        let converter = PCMConverter()
        let output = output
        input.installTap(onBus: 0, bufferSize: 2048, format: format) { buffer, _ in
            do { output.audio(1, try converter.convert(buffer)) }
            catch { output.error(error.localizedDescription) }
        }
        tapped = true
        tapFormat = format
        engine.prepare()
        try engine.start()
    }

    func stop() {
        engine.stop()
        if tapped { engine.inputNode.removeTap(onBus: 0); tapped = false }
        tapFormat = nil
    }
}

@available(macOS 13.0, *)
final class AudioCapture: NSObject, SCStreamOutput, SCStreamDelegate, @unchecked Sendable {
    private let output: Output
    private let microphone: MicrophoneInput
    private let microphoneQueue = DispatchQueue(label: "io.cuego.microphone-lifecycle")
    private var microphoneObserver: NSObjectProtocol?
    private lazy var recovery = MicrophoneRecovery(
        engine: microphone,
        schedule: { [weak self] action in
            self?.microphoneQueue.asyncAfter(deadline: .now() + .milliseconds(250), execute: action)
        },
        onState: { [weak self] in self?.output.diagnostic($0) },
        onError: { [weak self] in self?.output.error($0) })
    private let systemConverter = PCMConverter()
    private let systemQueue = DispatchQueue(label: "io.cuego.system-audio")
    private var stream: SCStream?

    init(output: Output) {
        self.output = output
        self.microphone = MicrophoneInput(output: output)
    }

    func start(system: Bool) async throws {
        let allowed: Bool
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized: allowed = true
        case .notDetermined: allowed = await AVCaptureDevice.requestAccess(for: .audio)
        default: allowed = false
        }
        guard allowed else { throw CaptureError.message("麦克风权限被拒绝，请到系统设置 → 隐私与安全性 → 麦克风中允许 Cue") }

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            microphoneQueue.async { [self] in
                microphoneObserver = NotificationCenter.default.addObserver(
                    forName: .AVAudioEngineConfigurationChange,
                    object: microphone.notificationObject,
                    queue: nil) { [weak self] _ in
                        guard let self else { return }
                        self.microphoneQueue.async { self.recovery.configurationChanged() }
                    }
                do {
                    try recovery.start()
                    continuation.resume()
                } catch {
                    if let observer = microphoneObserver { NotificationCenter.default.removeObserver(observer) }
                    microphoneObserver = nil
                    continuation.resume(throwing: error)
                }
            }
        }

        if system {
            let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
            guard let display = content.displays.first(where: { $0.displayID == CGMainDisplayID() }) ?? content.displays.first else {
                throw CaptureError.message("没有可用的屏幕，请检查屏幕录制权限")
            }
            let filter = SCContentFilter(display: display, excludingWindows: [])
            let config = SCStreamConfiguration()
            config.width = 2
            config.height = 2
            config.minimumFrameInterval = CMTime(value: 1, timescale: 1)
            config.capturesAudio = true
            config.excludesCurrentProcessAudio = true
            config.sampleRate = 16_000
            config.channelCount = 1
            let capture = SCStream(filter: filter, configuration: config, delegate: self)
            try capture.addStreamOutput(self, type: .audio, sampleHandlerQueue: systemQueue)
            // ScreenCaptureKit owns a display session even when only audio is
            // consumed; registering a screen sink avoids missing-output errors.
            try capture.addStreamOutput(self, type: .screen, sampleHandlerQueue: systemQueue)
            stream = capture
            try await capture.startCapture()
        }
        output.ready()
    }

    func stop() async {
        output.stop()
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            microphoneQueue.async { [self] in
                if let observer = microphoneObserver { NotificationCenter.default.removeObserver(observer) }
                microphoneObserver = nil
                recovery.stop()
                continuation.resume()
            }
        }
        if let stream { try? await stream.stopCapture() }
        stream = nil
    }

    func stream(_ stream: SCStream, didStopWithError error: Error) {
        output.error("系统音频采集停止：\(error.localizedDescription)")
    }

    func stream(_ stream: SCStream, didOutputSampleBuffer sampleBuffer: CMSampleBuffer, of type: SCStreamOutputType) {
        guard type == .audio, sampleBuffer.isValid,
              let description = CMSampleBufferGetFormatDescription(sampleBuffer),
              let asbd = CMAudioFormatDescriptionGetStreamBasicDescription(description),
              let format = AVAudioFormat(streamDescription: asbd) else { return }
        let frames = CMSampleBufferGetNumSamples(sampleBuffer)
        guard frames > 0,
              let pcm = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: AVAudioFrameCount(frames)) else { return }
        pcm.frameLength = AVAudioFrameCount(frames)
        let status = CMSampleBufferCopyPCMDataIntoAudioBufferList(sampleBuffer, at: 0, frameCount: Int32(frames), into: pcm.mutableAudioBufferList)
        guard status == noErr else { output.error("无法读取系统音频缓冲区（\(status)）"); return }
        do { output.audio(2, try systemConverter.convert(pcm)) }
        catch { output.error(error.localizedDescription) }
    }
}

@main
struct CaptureMain {
    static func main() async {
        let output = Output()
        guard #available(macOS 13.0, *) else {
            output.error("音频组件需要 macOS 13.0 或更新版本")
            Darwin.exit(1)
        }
        signal(SIGPIPE, SIG_IGN)
        signal(SIGINT, SIG_IGN)
        signal(SIGTERM, SIG_IGN)
        let capture = AudioCapture(output: output)
        let interrupts = [SIGINT, SIGTERM].map { code -> DispatchSourceSignal in
            let source = DispatchSource.makeSignalSource(signal: code, queue: .main)
            source.setEventHandler { Task { await capture.stop(); Darwin.exit(0) } }
            source.resume()
            return source
        }
        defer { for source in interrupts { source.cancel() } }
        do {
            try await capture.start(system: CommandLine.arguments.contains("--system"))
            // Keep the asynchronous main alive without polling or blocking the main queue.
            await withUnsafeContinuation { (_: UnsafeContinuation<Void, Never>) in }
        } catch {
            output.error(error.localizedDescription)
            await capture.stop()
            Darwin.exit(1)
        }
    }
}
