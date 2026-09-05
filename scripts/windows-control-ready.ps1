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
function Test-SettingsGearIcon([IntPtr]$Handle) {
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
            return $green -ge [Math]::Max(4, [Math]::Floor($pixels * 0.025)) -and $light -ge [Math]::Floor($pixels * 0.25)
        } finally {
            $bitmap.Dispose()
        }
    } finally {
        $icon.Dispose()
    }
}
$condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $AppProcessId)
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
while ((Get-Date) -lt $deadline) {
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    foreach ($window in $windows) {
        # Only inspect this test's control page; never traverse DSH conversation
        # content or output user data through UI Automation.
        if ($window.Current.Name -notlike '*Settings*') { continue }
        $status = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, $ExpectedStatus)
        if ($null -ne $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $status)) {
            # Verify actual gear pixels, not merely non-zero placeholder handles.
            $hwnd = [IntPtr]$window.Current.NativeWindowHandle
            $small = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]0, [IntPtr]0)
            $large = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]1, [IntPtr]0)
            if ($small -eq [IntPtr]::Zero -or $large -eq [IntPtr]::Zero) { continue }
            if (-not (Test-SettingsGearIcon $small) -or -not (Test-SettingsGearIcon $large)) { continue }
            Write-Output 'PASS: published Windows executable rendered real backend status through native UI Automation'
            Write-Output 'PASS: Settings native window small/large icons contain the expected green gear artwork'
            exit 0
        }
    }
    Start-Sleep -Milliseconds 500
}
throw "Published Windows executable did not render $ExpectedStatus with the expected gear icon"
