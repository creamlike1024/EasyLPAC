package easylpac.internal.usbhost

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.hardware.usb.UsbConstants
import android.hardware.usb.UsbDevice
import android.hardware.usb.UsbDeviceConnection
import android.hardware.usb.UsbManager
import android.os.Build
import dev.mygo.Native
import dev.mygo.NativeUnsupportedException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.coroutines.resume
import kotlinx.coroutines.suspendCancellableCoroutine

fun makeHost(): Host {
    val context = Native.context
    val manager = context.getSystemService(Context.USB_SERVICE) as? UsbManager
        ?: throw NativeUnsupportedException()
    return AndroidHost(context, manager)
}

private class AndroidHost(private val context: Context, private val manager: UsbManager) : Host {
    private val requests = AtomicInteger()

    override fun readers(): List<Reader> = manager.deviceList.values
        .filter(::isCCID)
        .sortedBy { it.deviceName }
        .map {
            Reader(
                name = it.deviceName,
                vendorID = it.vendorId.toLong(),
                productID = it.productId.toLong(),
                manufacturer = it.manufacturerName.orEmpty(),
                product = it.productName.orEmpty(),
            )
        }

    override suspend fun open(name: String): Connection {
        val device = manager.deviceList[name]?.takeIf(::isCCID) ?: throw UsbhostException(UsbhostError.GONE)
        if (!manager.hasPermission(device) && !requestPermission(device)) {
            // Unplugging while the dialog shows also answers it with no.
            val error = if (manager.deviceList.containsKey(name)) UsbhostError.DENIED else UsbhostError.GONE
            throw UsbhostException(error)
        }
        val connection = manager.openDevice(device) ?: throw UsbhostException(UsbhostError.GONE)
        return AndroidConnection(connection)
    }

    private suspend fun requestPermission(device: UsbDevice): Boolean = suspendCancellableCoroutine { cont ->
        val id = requests.incrementAndGet()
        val action = "${context.packageName}.USB_PERMISSION.$id"
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(c: Context, intent: Intent) {
                context.unregisterReceiver(this)
                val granted = intent.getBooleanExtra(UsbManager.EXTRA_PERMISSION_GRANTED, false)
                if (cont.isActive) cont.resume(granted || manager.hasPermission(device))
            }
        }
        val filter = IntentFilter(action)
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            context.registerReceiver(receiver, filter)
        }
        // Already unregistered when the answer came first.
        cont.invokeOnCancellation { runCatching { context.unregisterReceiver(receiver) } }
        // The system adds the answer as extras, so the intent is mutable,
        // and limited to this app.
        var flags = PendingIntent.FLAG_UPDATE_CURRENT
        if (Build.VERSION.SDK_INT >= 31) flags = flags or PendingIntent.FLAG_MUTABLE
        val answer = Intent(action).setPackage(context.packageName)
        manager.requestPermission(device, PendingIntent.getBroadcast(context, id, answer, flags))
    }
}

private fun isCCID(device: UsbDevice): Boolean = (0 until device.interfaceCount).any {
    val i = device.getInterface(it)
    i.interfaceClass == UsbConstants.USB_CLASS_CSCID && i.interfaceProtocol == 0
}

private class AndroidConnection(private var connection: UsbDeviceConnection?) : Connection {
    @Synchronized
    override fun fd(): Long {
        val fd = connection?.fileDescriptor ?: -1
        if (fd < 0) throw UsbhostException(UsbhostError.GONE)
        return fd.toLong()
    }

    @Synchronized
    override fun close() {
        connection?.close()
        connection = null
    }
}
