param([Parameter(Mandatory=$true)][int]$AppProcessId)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $AppProcessId)
$deadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $deadline) {
    $windows = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
    foreach ($window in $windows) {
        # Only inspect this test's control page. Its profile disables AutoStart,
        # so there is no DSH conversation content in the accessibility tree.
        if ($window.Current.Name -notlike '*Control Center*') { continue }
        $stopped = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, 'Stopped')
        if ($null -ne $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $stopped)) {
            Write-Output 'PASS: published Windows executable rendered real backend status through native UI Automation'
            exit 0
        }
    }
    Start-Sleep -Milliseconds 500
}
throw 'Published Windows executable did not render the Stopped backend status'
