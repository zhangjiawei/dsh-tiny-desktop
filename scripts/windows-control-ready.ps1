param([Parameter(Mandatory=$true)][int]$AppProcessId)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class SettingsIconProbe {
    [DllImport("user32.dll", CharSet=CharSet.Unicode)]
    public static extern IntPtr SendMessageW(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);
}
'@
$condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $AppProcessId)
$deadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $deadline) {
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    foreach ($window in $windows) {
        # Only inspect this test's control page. Its profile disables AutoStart,
        # so there is no DSH conversation content in the accessibility tree.
        if ($window.Current.Name -notlike '*Settings*') { continue }
        $stopped = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, 'Stopped')
        if ($null -ne $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $stopped)) {
            # WM_GETICON confirms both custom window icon sizes were installed.
            $hwnd = [IntPtr]$window.Current.NativeWindowHandle
            $small = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]0, [IntPtr]0)
            $large = [SettingsIconProbe]::SendMessageW($hwnd, 0x7F, [IntPtr]1, [IntPtr]0)
            if ($small -eq [IntPtr]::Zero -or $large -eq [IntPtr]::Zero) { continue }
            Write-Output 'PASS: published Windows executable rendered real backend status through native UI Automation'
            Write-Output 'PASS: Settings native window title and small/large icon handles'
            exit 0
        }
    }
    Start-Sleep -Milliseconds 500
}
throw 'Published Windows executable did not render the Stopped backend status'
