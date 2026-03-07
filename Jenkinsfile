pipeline {
    agent any

    environment {
        APP_NAME = "ginchat"
        TARGET_HOST = "192.168.74.128"
        TARGET_USER = "root"
        TARGET_PATH = "/home/muyu/deploy"
    }

    stages {

        stage('Checkout') {
            steps {
                cleanWs()       // 清空工作区
                git(
                    branch: 'master',
                    url: 'git@github.com:muyu0211/GoChat.git',
                    credentialsId: 'server-ssh-key'
                )
            }
        }

        stage('Build') {
            steps {
                script {
                    docker.image('golang:1.24.5').inside {
                        sh '''
                        go mod tidy
                        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${APP_NAME} ./cmd
                        '''
                    }
                }

                sh '''
                md5sum ${APP_NAME}
                '''
            }
        }

        stage('Deploy') {
            steps {
                sshagent(['server-ssh-key']) {
                    // 务必将外层改为双引号 """
                    sh """
                    set -x  # 开启打印执行过程，方便你明确知道它到底卡在了哪一行
                    
                    # 1. 明确打印出变量，如果变量为空，会在这里暴露出来
                    echo "Ready to deploy: \${APP_NAME}"
                    md5sum "\${APP_NAME}"

                    # 2. 停止服务
                    # 必须加 -n 参数！防止 ssh 在没有 TTY 的流水线中死等输入
                    # 必须加 BatchMode=yes！防止秘钥错误时等待密码
                    ssh -n -o StrictHostKeyChecking=no -o BatchMode=yes \${TARGET_USER}@\${TARGET_HOST} "
                        pkill -x \${APP_NAME} || true
                    "

                    # 3. 上传文件
                    scp -o StrictHostKeyChecking=no -o BatchMode=yes "\${APP_NAME}" \${TARGET_USER}@\${TARGET_HOST}:\${TARGET_PATH}/

                    # 4. 启动服务
                    # 必须加 -n 参数！
                    # nohup 后面必须加上 < /dev/null 彻底切断后台进程与 ssh 的联系
                    ssh -n -o StrictHostKeyChecking=no -o BatchMode=yes \${TARGET_USER}@\${TARGET_HOST} "
                        cd \${TARGET_PATH}
                        chmod +x ./\${APP_NAME}
                        nohup ./\${APP_NAME} > server.log 2>&1 < /dev/null &
                    "
                    """
                }
            }
        }
    }
}