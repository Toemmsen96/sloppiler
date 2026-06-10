package testfiles;
import java.util.Scanner;

public class Main {
	public static void main(String[] args) {
		Scanner scanner = new Scanner(System.in);
		System.out.print("input: ");
		String input = scanner.nextLine();
		if (input.contains("tomas")) {
			System.out.println("Du specko!");
			scanner.close();
			return;
		}else if (input.contains("sloppiler")) {
			System.out.println("Dismami!");
			scanner.close();
			return;
		}else{
			System.out.println("Hello " + input);
		}
		scanner.close();
	}
}
