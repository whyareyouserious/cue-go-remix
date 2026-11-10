package listening

import (
	"testing"
	"time"
)

func TestInputMetricsDistinguishMissingSilentAndAudible(t *testing.T) {
	start := time.Unix(100, 0)
	m := newInputMonitor(true, start, nil)
	got, warning := m.snapshot(start.Add(6*time.Second), false)
	if warning == nil || got[0].Frames != 0 || got[0].LastFrameMS != -1 {
		t.Fatal("未收到任何采集帧时缺少诊断")
	}
	if _, warning = m.snapshot(start.Add(8*time.Second), false); warning != nil {
		t.Fatal("相同采集告警不应重复弹出")
	}
	m = newInputMonitor(true, start, nil)
	m.observe("you", make([]int16, 1600), start.Add(6*time.Second))
	m.observe("them", []int16{-32768, 32767}, start.Add(6*time.Second))
	got, warning = m.snapshot(start.Add(6*time.Second), false)
	if warning == nil || got[0].Frames != 1 || got[0].AudioMS != 100 || got[0].Peak != 0 {
		t.Fatal("静音输入和无音频没有区分")
	}
	if got[1].Peak != 32768 || got[1].RMS < 32767 {
		t.Fatal("音量峰值溢出或双声道统计混淆")
	}
	m = newInputMonitor(false, start, nil)
	m.observe("you", voice(1600), start.Add(6*time.Second))
	got, warning = m.snapshot(start.Add(6*time.Second), false)
	if warning != nil || got[0].RMS != 2000 {
		t.Fatal("有效声音不应报告静音")
	}
	got, _ = m.snapshot(start.Add(7*time.Second), true)
	if got[0].Frames != 0 || got[0].LastFrameMS != 1000 || got[0].Phase != "audio-input-stop" {
		t.Fatal("统计窗口或最后一帧时间错误")
	}
}
