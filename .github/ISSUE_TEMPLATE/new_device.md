---
name: New Device Support
about: Request or contribute support for a new USB display
title: '[Device] '
labels: 'new-device'
---

## Device Information

**Manufacturer/Brand:**

**Model/Product Name:**

**Vendor ID (hex):** 0x

**Product ID (hex):** 0x

**Display Resolution:** x

**Purchase Link (if available):**

## USB Descriptor

```
Paste output of: lsusb -d XXXX:YYYY -v
```

## Protocol Information (if known)

- [ ] I have USB traffic captures
- [ ] I've identified the pixel format (RGB565/RGB888)
- [ ] I've identified the byte order (big/little endian)
- [ ] I've identified the command structure

## Contribution

- [ ] I'm willing to help test
- [ ] I'm willing to implement the device profile
- [ ] I can provide USB captures

## Additional Notes

Any other information about the device, similar devices, or existing software that supports it.
