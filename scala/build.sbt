name := "gamestats-scala"
version := "0.1"
scalaVersion := "2.13.12"
Compile / mainClass := Some("Main")

// Runtime pequeño para Docker, sin sbt ni compilador en la imagen final.
val stageRuntime = taskKey[File]("Preparar aplicación y dependencias para java -cp")
stageRuntime := {
  val destination = target.value / "runtime"
  IO.createDirectory(destination)
  val application = (Compile / packageBin).value
  val dependencies = (Compile / dependencyClasspath).value.files
  (application +: dependencies).foreach(file => IO.copyFile(file, destination / file.getName))
  destination
}

libraryDependencies ++= Seq(
  "com.lihaoyi" %% "upickle" % "3.1.0",
  "com.lihaoyi" %% "os-lib" % "0.9.1",
  "org.scalameta" %% "munit" % "0.7.29" % Test
)

testFrameworks += new TestFramework("munit.Framework")
