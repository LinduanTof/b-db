class Dbrix < Formula
  desc "Terminal database client for PostgreSQL, MySQL, SQLite, Redis and MongoDB"
  homepage "https://github.com/LinduanTof/dbrix"
  version "0.3.2"
  license "Apache-2.0"

  depends_on :linux

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/LinduanTof/dbrix/releases/download/v#{version}/dbrix_#{version}_linux_arm64.tar.gz"
      sha256 "2e08293a50575c4a81dc391514c8188a1095a978a854c1b5cafa70ba2b814e68"
    else
      url "https://github.com/LinduanTof/dbrix/releases/download/v#{version}/dbrix_#{version}_linux_amd64.tar.gz"
      sha256 "1f3d21014b26f2377695e31b3f8966b8129663874048405806644f6941ac476c"
    end
  end

  def install
    bin.install "dbrix"
  end

  test do
    assert_match "dbrix", shell_output("#{bin}/dbrix --version")
  end
end
