// .majordomo/lib/logger.groovy
// Logging utilities — injected into stages via dependency injection (IoC)

def info(String message) {
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [INFO] ${message}"
}

def warn(String message) {
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [WARN] ${message}"
}

def error(String message) {
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [ERROR] ${message}"
}

def header(String message) {
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [INFO] ========== ${message} =========="
}

return this
