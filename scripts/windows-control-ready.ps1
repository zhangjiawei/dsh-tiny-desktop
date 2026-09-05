param(
    [Parameter(Mandatory=$true)][int]$AppProcessId,
    [string]$ExpectedStatus = 'Stopped',
    [int]$TimeoutSeconds = 45
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class SettingsIconProbe {
    [DllImport("user32.dll", CharSet=CharSet.Unicode)]
    public static extern IntPtr SendMessageW(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);
}
'@
function Get-SettingsGearIconStats([IntPtr]$Handle) {
    $source = [System.Drawing.Icon]::FromHandle($Handle)
    $icon = [System.Drawing.Icon]$source.Clone()
    try {
        $bitmap = $icon.ToBitmap()
        try {
            $green = 0
            $light = 0
            for ($x = 0; $x -lt $bitmap.Width; $x++) {
                for ($y = 0; $y -lt $bitmap.Height; $y++) {
                    $pixel = $bitmap.GetPixel($x, $y)
                    if ($pixel.A -gt 0 -and $pixel.G -gt ($pixel.R + 30) -and $pixel.G -gt ($pixel.B + 20)) { $green++ }
                    if ($pixel.A -gt 0 -and $pixel.R -gt 220 -and $pixel.G -gt 225 -and $pixel.B -gt 220) { $light++ }
                }
            }
            $pixels = $bitmap.Width * $bitmap.Height
            return [PSCustomObject]@{
                Green = $green
                Light = $light
                Pixels = $pixels
                # The accent survives Windows' 16 px caption resampling and is
                # enough to distinguish this icon from the neutral placeholder.
                HasGreenMark = $green -ge [Math]::Max(4, [Math]::Floor($pixels * 0.02))
                # Validate the pale gear field on the 32 px large icon, where
                # that detail is guaranteed to survive system resampling.
                HasLightDetail = $light -ge [Math]::Floor($pixels * 0.08)
            }
        } finally {
            $bitmap.Dispose()
        }
    } finally {
        $icon.Dispose()
    }
}
$condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $AppProcessId)
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$expectedSeenAt = $null
$lastStatus = 'not found'
$lastIcon = 'not inspected'
while ((Get-Date) -lt $deadline) {
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    foreach ($window in $windows) {
        # Only inspect this test's control page; never traverse DSH conversation
        # content or output user data through UI Automation.
        if ($window.Current.Name -notlike '*Settings*') { continue }
        foreach ($phase in @('Stopped', 'Preparing runtime', 'Starting and authenticating', 'Running', 'Needs attention')) {
            $phaseCondition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, $phase)
            if ($null -ne $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $phaseCondition)) {
                $lastStatus = $phase
                break
            }
        }
        if ($lastStatus -eq 'Needs attention') { throw 'Published Windows executable reached Needs attention during fresh GUI install' }
        $status = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, $ExpectedStatus)
        if ($null -ne $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $status)) {
            # Verify actual gear pixels, not merely non-zero placeholder handles.
            $hwnd = [IntPtr]$window.Current.NativeWindowHandle
            $small = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]0, [IntPtr]0)
            $large = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]1, [IntPtr]0)
            $small2 = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]2, [IntPtr]0)
            # WM_GETICON explicitly permits a zero result and documents the
            # alternate small-icon/class fallback. Different Windows builds
            # may expose the same application-provided caption icon through
            # ICON_SMALL, ICON_SMALL2, or both; validate every returned handle
            # without incorrectly requiring both slots to exist.
            if (($small -ne [IntPtr]::Zero -or $small2 -ne [IntPtr]::Zero) -and $large -ne [IntPtr]::Zero) {
                $smallStats = if ($small -ne [IntPtr]::Zero) { Get-SettingsGearIconStats $small } else { $null }
                $small2Stats = if ($small2 -ne [IntPtr]::Zero) { Get-SettingsGearIconStats $small2 } else { $null }
                $largeStats = Get-SettingsGearIconStats $large
                $smallText = if ($null -eq $smallStats) { 'none' } else { "$($smallStats.Green)/$($smallStats.Light)/$($smallStats.Pixels)" }
                $small2Text = if ($null -eq $small2Stats) { 'none' } else { "$($small2Stats.Green)/$($small2Stats.Light)/$($small2Stats.Pixels)" }
                $lastIcon = "small=$smallText, small2=$small2Text, large=$($largeStats.Green)/$($largeStats.Light)/$($largeStats.Pixels)"
                # Windows may reduce the 16 px light strokes to zero pixels,
                # while the five-pixel green gear remains visibly distinct.
                # Keep requiring every returned caption slot to carry that
                # custom accent, and require both signals on the large icon.
                $smallValid = ($null -eq $smallStats -or $smallStats.HasGreenMark) -and ($null -eq $small2Stats -or $small2Stats.HasGreenMark)
                $largeValid = $largeStats.HasGreenMark -and $largeStats.HasLightDetail
                if ($smallValid -and $largeValid) {
                    Write-Output 'PASS: published Windows executable rendered real backend status through native UI Automation'
                    Write-Output "PASS: Settings caption accent and large gear artwork verified ($lastIcon)"
                    exit 0
                }
            } else {
                $lastIcon = "handles small=$small small2=$small2 large=$large"
            }
            if ($null -eq $expectedSeenAt) { $expectedSeenAt = Get-Date }
            if (((Get-Date) - $expectedSeenAt).TotalSeconds -ge 10) {
                throw "Published Windows executable rendered $ExpectedStatus but its native gear icon was wrong: $lastIcon"
            }
        }
    }
    Start-Sleep -Milliseconds 500
}
throw "Published Windows executable did not render $ExpectedStatus with the expected gear icon; last status=$lastStatus; icon=$lastIcon"
