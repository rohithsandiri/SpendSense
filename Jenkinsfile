pipeline {
    agent any

    environment {
        PATH = "/usr/local/bin:/opt/homebrew/bin:/bin:/usr/bin:/usr/sbin:/sbin:${env.PATH}"
        DOCKER_HUB_REPO = 'yuvraj12345678/spendsense'
        BACKEND_IMAGE = "${DOCKER_HUB_REPO}-backend"
        FRONTEND_IMAGE = "${DOCKER_HUB_REPO}-frontend"
    }

    stages {
        stage('Checkout') {
            steps {
                git branch: 'main', url: 'https://github.com/rohithsandiri/SpendSense.git'
            }
        }

        stage('Run Tests') {
            steps {
                dir('backend') {
                    sh 'go build ./...'
                    sh 'go vet ./...'
                    sh 'go test ./...'
                }
                dir('frontend') {
                    sh 'npm install'
                    sh 'npm test -- --watchAll=false'
                }
            }
        }

        stage('Build Docker Images') {
            steps {
                sh "docker build -t ${BACKEND_IMAGE}:${env.BUILD_NUMBER} ./backend"
                sh "docker build -t ${FRONTEND_IMAGE}:${env.BUILD_NUMBER} ./frontend"
            }
        }

        stage('Push to Docker Hub') {
            steps {
                // Securely fetch Docker Hub credentials from HashiCorp Vault
                // Requires Jenkins HashiCorp Vault Plugin
                /*
                withVault(configuration: [vaultUrl: 'http://vault:8200', vaultCredentialId: 'vault-token'],
                          secrets: [[path: 'secret/data/spendsense/dockerhub', secretValues: [
                              [envVar: 'DOCKER_HUB_USER', vaultKey: 'username'],
                              [envVar: 'DOCKER_HUB_PWD', vaultKey: 'password']
                          ]]]) {
                    sh "docker login -u ${DOCKER_HUB_USER} -p ${DOCKER_HUB_PWD}"
                }
                */
                // For now, assuming pre-authenticated or using standard credentials
                sh "docker push ${BACKEND_IMAGE}:${env.BUILD_NUMBER}"
                sh "docker push ${FRONTEND_IMAGE}:${env.BUILD_NUMBER}"
                sh "docker tag ${BACKEND_IMAGE}:${env.BUILD_NUMBER} ${BACKEND_IMAGE}:latest"
                sh "docker push ${BACKEND_IMAGE}:latest"
                sh "docker tag ${FRONTEND_IMAGE}:${env.BUILD_NUMBER} ${FRONTEND_IMAGE}:latest"
                sh "docker push ${FRONTEND_IMAGE}:latest"
            }
        }

        stage('Deploy') {
            steps {
                script {
                    sh 'kubectl apply -f k8s/'
                }
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}