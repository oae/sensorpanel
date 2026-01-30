# Get full details for the USB-Display device
$device = Get-PnpDevice -PresentOnly | Where-Object { $_.InstanceId -like '*1908*0102*' }
if ($device) {
    Write-Host "USB-Display found:"
    $device | Format-List *
    
    Write-Host "`nFull Instance ID:"
    $device.InstanceId
    
    # Get parent device info
    Write-Host "`nDevice location path:"
    Get-PnpDeviceProperty -InstanceId $device.InstanceId -KeyName DEVPKEY_Device_LocationPaths | Select-Object -ExpandProperty Data
}

# List USBPcap devices if installed
Write-Host "`n`nLooking for USBPcap:"
if (Test-Path "C:\Program Files\USBPcap") {
    Write-Host "USBPcap found at C:\Program Files\USBPcap"
    & "C:\Program Files\USBPcap\USBPcapCMD.exe" -d
} elseif (Test-Path "C:\Program Files (x86)\USBPcap") {
    Write-Host "USBPcap found at C:\Program Files (x86)\USBPcap"
    & "C:\Program Files (x86)\USBPcap\USBPcapCMD.exe" -d
} else {
    Write-Host "USBPcap not found - check Wireshark installation"
}
