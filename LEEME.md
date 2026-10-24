# Control Remoto

Mové el mouse de tu PC con Windows desde el celular Android: touchpad, clics, arrastrar,
desplazamiento, escribir texto, multimedia y presentaciones. Funciona por Wi‑Fi en casa y
por internet con Tailscale.

```
control-remoto/
├─ pc/                 Programa de Windows (Go, sin dependencias)
│  ├─ main.go          Servidor, conexión y teclas permitidas
│  ├─ input_windows.go Movimiento de mouse y teclado (SendInput de Windows)
│  ├─ input_other.go   Modo simulado para probar fuera de Windows
│  └─ web/index.html   Interfaz del celular (la usa la app y el navegador)
├─ android/            App de Android (abre la misma interfaz)
└─ .github/workflows/  Compila el .exe y el .apk automáticamente en GitHub
```

## 1. Programa de Windows

1. Abrí `ControlRemoto.exe`. La primera vez Windows puede mostrar:
   - **"Windows protegió tu PC"** (SmartScreen): tocá *Más información → Ejecutar de todas formas*. Aparece porque el programa no está firmado por una empresa.
   - **Firewall de Windows**: marcá **Redes privadas** y tocá *Permitir*. Sin esto, el celular no puede conectarse.
2. La ventana muestra el **código de vinculación**, la **IP Wi‑Fi** y la **IP Tailscale**. Dejala abierta mientras uses el control.

El código se guarda y se mantiene entre usos. Para generar uno nuevo:
`ControlRemoto.exe -nuevo-codigo` (también se puede cambiar el puerto con `-puerto 9000`).

> Si tu red Wi‑Fi figura como **Pública** en Windows, el firewall puede bloquear la conexión.
> Cambiala a Privada en *Configuración → Red e Internet → Wi‑Fi → (tu red) → Tipo de perfil de red*.

## 2. App de Android (.apk)

### Opción A: compilarla gratis en GitHub (no instalás nada)

1. Creá una cuenta en github.com y un repositorio nuevo **privado** (por ejemplo `control-remoto`).
2. En el repositorio: *Add file → Upload files* y arrastrá **todo el contenido** de esta carpeta. Confirmá con *Commit changes*.
3. Verificá que se haya subido `.github/workflows/compilar.yml` (algunas veces las carpetas que empiezan con punto no se suben). Si falta: pestaña *Actions → set up a workflow yourself*, pegá el contenido de ese archivo y guardalo con ese nombre.
4. Andá a la pestaña **Actions**. Vas a ver "Compilar" ejecutándose (tarda unos 5 minutos).
5. Cuando termine con tilde verde, entrá a la ejecución y en **Artifacts** descargá `ControlRemoto-Android` (adentro está `ControlRemoto.apk`). También está `ControlRemoto-Windows` con el .exe.

### Opción B: con Android Studio

Abrí la carpeta `android` en Android Studio y elegí *Build → Build App Bundle(s) / APK(s) → Build APK(s)*.

### Instalar el APK en el celular

Pasá el archivo al celular (WhatsApp a vos mismo, Drive o cable), abrilo y aceptá
"instalar apps de origen desconocido" cuando lo pida. Si Play Protect avisa, tocá *Instalar de todas formas*.

## 3. Primera conexión

1. Celular y PC en la misma red Wi‑Fi, programa abierto en la PC.
2. Abrí la app: completá **IP Wi‑Fi** y **código** con lo que muestra la PC, y tocá *Guardar y conectar*.
3. El punto verde arriba indica que está conectado.

**Sin instalar la app**: también funciona desde el navegador del celular, abriendo la dirección
que muestra el programa (`http://IP:8765/?c=CÓDIGO`).

## 4. Usarlo por internet con Tailscale

1. Instalá Tailscale en la PC (tailscale.com/download) e iniciá sesión.
2. Instalá **Tailscale** en el celular desde Play Store con **la misma cuenta**.
3. Reiniciá `ControlRemoto.exe`: va a mostrar la **IP Tailscale** (empieza con `100.`).
4. Cargala en la app, en "IP Tailscale". La app prueba primero el Wi‑Fi y, si no responde, Tailscale.

Fuera de casa, Tailscale tiene que estar activo en los dos equipos. Por internet vas a notar algo
más de demora que en Wi‑Fi; es normal.

## Gestos

| Acción | Resultado |
|---|---|
| Deslizar 1 dedo | Mover el puntero (más rápido = va más lejos) |
| Tocar con 1 dedo | Clic izquierdo |
| Tocar con 2 dedos | Clic derecho |
| Deslizar 2 dedos | Desplazar (scroll) |
| Botón Arrastrar | Mantiene apretado el clic izquierdo hasta tocar "Soltar" |

## Seguridad

- Sólo acepta conexiones con el código de 6 números; tras 5 intentos fallidos bloquea esa dirección 1 minuto.
- No está expuesto a internet: por fuera de tu Wi‑Fi sólo se llega a través de tu red privada de Tailscale.
- La clave de firma del APK (`android/app/controlremoto.jks`) está incluida para que puedas actualizar
  la app sin desinstalarla. Mantené el repositorio **privado**.
